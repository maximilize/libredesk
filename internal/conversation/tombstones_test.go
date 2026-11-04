package conversation

import (
	"testing"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/zerodha/logf"
)

func TestDeleteConversationLeavesTombstones(t *testing.T) {
	db := testutil.NewDB(t, "tombstones")
	var contactID, inboxID int
	if err := db.Get(&contactID, `INSERT INTO users (type, email, first_name, last_name) VALUES ('contact', 'customer@example.com', 'Clara', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&inboxID, `INSERT INTO inboxes (name, channel) VALUES ('Mail', 'email') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	var uuid string
	if err := db.Get(&uuid, `INSERT INTO conversations (contact_id, inbox_id, status_id) VALUES ($1, $2, (SELECT id FROM conversation_statuses WHERE name = 'Open')) RETURNING uuid`, contactID, inboxID); err != nil {
		t.Fatal(err)
	}
	for _, sourceID := range []string{"<first@example.com>", "<second@example.com>", ""} {
		if _, err := db.Exec(`INSERT INTO conversation_messages (type, status, conversation_id, sender_id, sender_type, content, content_type, source_id)
			VALUES ('incoming', 'received', (SELECT id FROM conversations WHERE uuid = $1), $2, 'contact', 'Hello', 'text', NULLIF($3, ''))`, uuid, contactID, sourceID); err != nil {
			t.Fatal(err)
		}
	}
	lo := logf.New(logf.Opts{})
	m := &Manager{lo: &lo, db: db, i18n: testutil.NewI18n(t)}
	if err := dbutil.ScanSQLFile("queries.sql", &m.q, db, efs); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"<first@example.com>", "<second@example.com>"} {
		if exists, err := m.MessageExists(id); err != nil || !exists {
			t.Fatalf("before delete: MessageExists(%s) = %v, %v; want true", id, exists, err)
		}
	}
	if err := m.DeleteConversation(uuid); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM deleted_message_source_ids`); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("tombstones = %d, want 2", count)
	}
	// The mails are still on the server; the ingest must treat them as already seen.
	for _, id := range []string{"<first@example.com>", "<second@example.com>"} {
		if exists, err := m.MessageExists(id); err != nil || !exists {
			t.Fatalf("after delete: MessageExists(%s) = %v, %v; want true", id, exists, err)
		}
	}
	if exists, err := m.MessageExists("<never@example.com>"); err != nil || exists {
		t.Fatalf("unknown id: MessageExists = %v, %v; want false", exists, err)
	}
	// Old tombstones are swept, fresh ones stay.
	if _, err := db.Exec(`UPDATE deleted_message_source_ids SET deleted_at = NOW() - INTERVAL '91 days' WHERE source_id = '<first@example.com>'`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.q.DeleteOldMessageTombstones.Exec(); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&count, `SELECT COUNT(*) FROM deleted_message_source_ids`); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("tombstones after sweep = %d, want 1", count)
	}
}
