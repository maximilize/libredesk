package conversation

import (
	"fmt"
	"slices"
	"testing"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/zerodha/logf"
)

func subjectFilter(operator, value string) string {
	return fmt.Sprintf(`[{"model":"conversations","field":"subject","operator":%q,"value":%q}]`, operator, value)
}

func TestViewFiltersAcceptSubject(t *testing.T) {
	if err := dbutil.ValidateFilters(subjectFilter("contains", "alert"), ListFilterAllowedFields, ListFilterRenderers); err != nil {
		t.Fatalf("expected subject filter to be valid, got %v", err)
	}
}

func TestConversationListSubjectFilter(t *testing.T) {
	db := testutil.NewDB(t, "subject_filter")
	lo := logf.New(logf.Opts{})
	m := &Manager{lo: &lo, i18n: testutil.NewI18n(t), settingsStore: stubSettingsStore{}, db: db}
	if err := dbutil.ScanSQLFile("queries.sql", &m.q, db, efs); err != nil {
		t.Fatal(err)
	}

	var contact, agent, inbox int
	if err := db.Get(&contact, `INSERT INTO users (type, first_name, last_name, email) VALUES ('contact', 'Clara', 'Huber', 'clara@example.com') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&agent, `INSERT INTO users (type, first_name, last_name) VALUES ('agent', 'Agent', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&inbox, `INSERT INTO inboxes (name, channel) VALUES ('Inbox', 'email') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		subject  string
		assignee int
	}{
		{"Panel alert: disk usage", agent},
		{"Panel alert: disk usage", 0},
		{"Re: panel ALERT: disk usage", 0},
		{"Invoice for October", 0},
		{"Upgrade 100% done", 0},
		{"Upgrade 1000 done", 0},
	} {
		if _, err := db.Exec(`INSERT INTO conversations (contact_id, inbox_id, status_id, subject, assigned_user_id)
		VALUES ($1, $2, (SELECT id FROM conversation_statuses LIMIT 1), $3, NULLIF($4, 0))`, contact, inbox, c.subject, c.assignee); err != nil {
			t.Fatal(err)
		}
	}

	subjects := func(t *testing.T, filters string, assignedTo int) []string {
		t.Helper()
		var (
			got []string
			err error
		)
		if assignedTo != 0 {
			list, e := m.GetAssignedConversationsList(agent, assignedTo, "", "", filters, 1, 50)
			for _, c := range list {
				got = append(got, c.Subject.String)
			}
			err = e
		} else {
			list, e := m.GetAllConversationsList(agent, "", "", filters, 1, 50)
			for _, c := range list {
				got = append(got, c.Subject.String)
			}
			err = e
		}
		if err != nil {
			t.Fatalf("listing conversations: %v", err)
		}
		slices.Sort(got)
		return got
	}

	t.Run("contains is case-insensitive", func(t *testing.T) {
		got := subjects(t, subjectFilter("contains", "panel alert"), 0)
		want := []string{"Panel alert: disk usage", "Panel alert: disk usage", "Re: panel ALERT: disk usage"}
		if !slices.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("not contains", func(t *testing.T) {
		got := subjects(t, subjectFilter("not contains", "alert"), 0)
		want := []string{"Invoice for October", "Upgrade 100% done", "Upgrade 1000 done"}
		if !slices.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("wildcards match literally", func(t *testing.T) {
		got := subjects(t, subjectFilter("contains", "100%"), 0)
		if !slices.Equal(got, []string{"Upgrade 100% done"}) {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("list scope still applies", func(t *testing.T) {
		got := subjects(t, subjectFilter("contains", "panel alert"), agent)
		if !slices.Equal(got, []string{"Panel alert: disk usage"}) {
			t.Fatalf("assigned list returned %q", got)
		}
	})
}
