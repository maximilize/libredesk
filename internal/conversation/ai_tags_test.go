package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/zerodha/logf"
)

type fakeTagSuggester struct {
	transcript string
	calls      int
}

func (f *fakeTagSuggester) SuggestTags(_ context.Context, transcript string) ([]string, error) {
	f.calls++
	f.transcript = transcript
	return []string{"payment"}, nil
}

func TestAITags(t *testing.T) {
	db := testutil.NewDB(t, "ai_tags")
	var contactID, inboxID int
	if err := db.Get(&contactID, `INSERT INTO users (type, email, first_name, last_name) VALUES ('contact', 'customer@example.com', 'Clara', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&inboxID, `INSERT INTO inboxes (name, channel) VALUES ('Mail', 'email') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	newConversation := func() string {
		var uuid string
		if err := db.Get(&uuid, `INSERT INTO conversations (contact_id, inbox_id, status_id) VALUES ($1, $2, (SELECT id FROM conversation_statuses WHERE name = 'Open')) RETURNING uuid`, contactID, inboxID); err != nil {
			t.Fatal(err)
		}
		return uuid
	}
	lo := logf.New(logf.Opts{})
	m := &Manager{lo: &lo, db: db, i18n: testutil.NewI18n(t)}
	if err := dbutil.ScanSQLFile("queries.sql", &m.q, db, efs); err != nil {
		t.Fatal(err)
	}

	if _, err := m.AITags(context.Background(), newConversation()); err == nil {
		t.Fatal("expected an error without a suggester")
	}
	suggester := &fakeTagSuggester{}
	m.SetTagSuggester(suggester)

	// Nothing to read yet: no call to the AI.
	empty := newConversation()
	if tags, err := m.AITags(context.Background(), empty); err != nil || tags != nil || suggester.calls != 0 {
		t.Fatalf("empty conversation = %v, %v, calls %d; want nil, nil, 0", tags, err, suggester.calls)
	}

	// Public messages make the transcript, the private note stays out.
	uuid := newConversation()
	for _, row := range []struct {
		typ, status, sender string
		private             bool
		text                string
	}{
		{"incoming", "received", "contact", false, "My premium was charged twice"},
		{"outgoing", "sent", "agent", false, "We are looking into the charge"},
		{"outgoing", "sent", "agent", true, "internal: check the invoice"},
	} {
		if _, err := db.Exec(`INSERT INTO conversation_messages (type, status, conversation_id, sender_id, sender_type, content, text_content, content_type, private)
			VALUES ($1, $2, (SELECT id FROM conversations WHERE uuid = $3), $4, $5, $6, $6, 'text', $7)`, row.typ, row.status, uuid, contactID, row.sender, row.text, row.private); err != nil {
			t.Fatal(err)
		}
	}
	tags, err := m.AITags(context.Background(), uuid)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "payment" || suggester.calls != 1 {
		t.Fatalf("tags = %v, calls %d; want [payment], 1", tags, suggester.calls)
	}
	for _, want := range []string{"charged twice", "looking into the charge"} {
		if !strings.Contains(suggester.transcript, want) {
			t.Errorf("transcript misses %q:\n%s", want, suggester.transcript)
		}
	}
	if strings.Contains(suggester.transcript, "check the invoice") {
		t.Errorf("transcript contains the private note:\n%s", suggester.transcript)
	}
}
