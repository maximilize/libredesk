package conversation

import (
	"encoding/json"
	"testing"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/inbox"
	"github.com/abhinavxd/libredesk/internal/template"
	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/abhinavxd/libredesk/internal/user"
	umodels "github.com/abhinavxd/libredesk/internal/user/models"
	"github.com/abhinavxd/libredesk/internal/ws"
	wsmodels "github.com/abhinavxd/libredesk/internal/ws/models"
	"github.com/zerodha/logf"
)

func TestUpdateConversationInbox(t *testing.T) {
	db := testutil.NewDB(t, "conversation_inbox_move")
	lo := logf.New(logf.Opts{Level: logf.FatalLevel})
	lang := testutil.NewI18n(t)
	inboxes, err := inbox.New(&lo, db, lang, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	users, err := user.New(lang, user.Opts{DB: db, Lo: &lo})
	if err != nil {
		t.Fatal(err)
	}
	hub := ws.NewHub(&lo, nil /** userStore **/)
	m := &Manager{
		db: db, lo: &lo, i18n: lang, inboxStore: inboxes, userStore: users,
		template: &template.Manager{}, mediaStore: whatsAppReplyMedia{},
		webhookStore: whatsAppReplyWebhook{}, wsHub: hub,
	}
	if err := dbutil.ScanSQLFile("queries.sql", &m.q, db, efs); err != nil {
		t.Fatal(err)
	}

	newInbox := func(name, channel, from string, enabled bool) int {
		t.Helper()
		var id int
		if err := db.Get(&id, `INSERT INTO inboxes (name, channel, "from", enabled) VALUES ($1, $2, $3, $4) RETURNING id`,
			name, channel, from, enabled); err != nil {
			t.Fatal(err)
		}
		return id
	}
	technical := newInbox("Technical", "email", "technical@example.com", true)
	billing := newInbox("Billing", "email", "billing@example.com", true)
	disabled := newInbox("Old", "email", "old@example.com", false)
	chat := newInbox("Chat", "livechat", "", true)

	var contactID int
	if err := db.Get(&contactID, `INSERT INTO users (type, email, first_name, last_name)
		VALUES ('contact', 'customer@example.com', 'Customer', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	actor := umodels.User{FirstName: "Agent", LastName: "One"}
	if err := db.Get(&actor.ID, `INSERT INTO users (type, email, first_name, last_name)
		VALUES ('agent', 'agent@example.com', 'Agent', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}

	newConversation := func(inboxID int) string {
		t.Helper()
		var uuid string
		if err := db.Get(&uuid, `INSERT INTO conversations (contact_id, inbox_id, status_id)
			VALUES ($1, $2, (SELECT id FROM conversation_statuses LIMIT 1)) RETURNING uuid`, contactID, inboxID); err != nil {
			t.Fatal(err)
		}
		return uuid
	}
	inboxOf := func(uuid string) int {
		t.Helper()
		var id int
		if err := db.Get(&id, `SELECT inbox_id FROM conversations WHERE uuid = $1`, uuid); err != nil {
			t.Fatal(err)
		}
		return id
	}
	activities := func(uuid string) []string {
		t.Helper()
		var content []string
		if err := db.Select(&content, `SELECT m.content FROM conversation_messages m
			JOIN conversations c ON c.id = m.conversation_id
			WHERE c.uuid = $1 AND m.type = 'activity' ORDER BY m.id`, uuid); err != nil {
			t.Fatal(err)
		}
		return content
	}

	t.Run("moves to another email inbox", func(t *testing.T) {
		conv := newConversation(technical)
		client := &ws.Client{Send: make(chan wsmodels.WSMessage, 8)}
		hub.SubscribeOpenConv(client, conv)

		if err := m.UpdateConversationInbox(conv, billing, actor); err != nil {
			t.Fatal(err)
		}
		if got := inboxOf(conv); got != billing {
			t.Fatalf("inbox = %d, want %d", got, billing)
		}
		if got := activities(conv); len(got) != 1 || got[0] != "Agent One moved the conversation to the Billing inbox" {
			t.Fatalf("activities = %q", got)
		}

		var update map[string]any
		for len(client.Send) > 0 {
			msg := <-client.Send
			var payload struct {
				Type string         `json:"type"`
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(msg.Data, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Type == wsmodels.MessageTypeConversationUpdate {
				update = payload.Data
			}
		}
		if update == nil || update["inbox_id"] != float64(billing) || update["inbox_name"] != "Billing" || update["inbox_mail"] != "billing@example.com" {
			t.Fatalf("conversation update = %v", update)
		}

		// Moving to the inbox it is already in is a no-op.
		if err := m.UpdateConversationInbox(conv, billing, actor); err != nil {
			t.Fatal(err)
		}
		if got := activities(conv); len(got) != 1 {
			t.Fatalf("no-op move recorded an activity: %q", got)
		}
	})

	rejected := []struct {
		name         string
		from, target int
	}{
		{"disabled inbox", technical, disabled},
		{"live chat inbox", technical, chat},
		{"missing inbox", technical, 999999},
		{"live chat conversation", chat, technical},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			conv := newConversation(tc.from)
			if err := m.UpdateConversationInbox(conv, tc.target, actor); err == nil {
				t.Fatal("move accepted")
			}
			if got := inboxOf(conv); got != tc.from {
				t.Fatalf("inbox = %d, want unchanged %d", got, tc.from)
			}
			if got := activities(conv); len(got) != 0 {
				t.Fatalf("rejected move recorded an activity: %q", got)
			}
		})
	}
}
