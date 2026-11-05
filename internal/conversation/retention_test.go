package conversation

import (
	"context"
	"fmt"
	"testing"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/zerodha/logf"
)

func TestDeleteExpiredConversations(t *testing.T) {
	db := testutil.NewDB(t, "retention")
	var contactID, inboxID int
	if err := db.Get(&contactID, `INSERT INTO users (type, email, first_name, last_name) VALUES ('contact', 'customer@example.com', 'Customer', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&inboxID, `INSERT INTO inboxes (name, channel) VALUES ('Mail', 'email') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	newContact := func(email string) int {
		var id int
		if err := db.Get(&id, `INSERT INTO users (type, email, first_name, last_name) VALUES ('contact', $1, 'Someone', '') RETURNING id`, email); err != nil {
			t.Fatal(err)
		}
		return id
	}
	// insertFor creates a conversation with one message for the contact, in the given status, with each timestamp an interval back from now.
	insertFor := func(contactID int, status, createdAgo, lastMessageAgo, lastResolvedAgo string) int {
		var id int
		if err := db.Get(&id, `INSERT INTO conversations (contact_id, inbox_id, status_id, created_at, last_message_at, last_resolved_at)
			VALUES ($1, $2, (SELECT id FROM conversation_statuses WHERE name = $3), NOW() - $4::interval, NOW() - $5::interval, NOW() - $6::interval) RETURNING id`,
			contactID, inboxID, status, createdAgo, lastMessageAgo, lastResolvedAgo); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO conversation_messages (type, status, conversation_id, sender_id, sender_type, content, created_at, source_id) VALUES ('incoming', 'received', $1, $2, 'contact', 'Hello', NOW() - $3::interval, $4)`, id, contactID, lastMessageAgo, fmt.Sprintf("<%d@example.com>", id)); err != nil {
			t.Fatal(err)
		}
		return id
	}
	insert := func(status, createdAgo, lastMessageAgo, lastResolvedAgo string) int {
		return insertFor(contactID, status, createdAgo, lastMessageAgo, lastResolvedAgo)
	}
	exists := func(table string, id int) bool {
		var found bool
		if err := db.Get(&found, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = $1)`, id); err != nil {
			t.Fatal(err)
		}
		return found
	}
	// A contact whose only conversation expires goes with it, the main contact keeps their open conversation.
	lonelyContact := newContact("lonely@example.com")
	lonelyExpired := insertFor(lonelyContact, "Closed", "90 days", "60 days", "45 days")
	// A message that arrived after the recorded last message means its timestamp update is still on the way.
	messageInFlight := insert("Closed", "90 days", "60 days", "45 days")
	if _, err := db.Exec(`INSERT INTO conversation_messages (type, status, conversation_id, sender_id, sender_type, content) VALUES ('incoming', 'received', $1, $2, 'contact', 'One more thing')`, messageInFlight, contactID); err != nil {
		t.Fatal(err)
	}
	// A contact who also wrote in someone else's conversation keeps their messages, so they stay.
	writerContact := newContact("writer@example.com")
	writerExpired := insertFor(writerContact, "Closed", "90 days", "60 days", "45 days")
	expiredClosed := insert("Closed", "90 days", "60 days", "45 days")
	expiredResolved := insert("Resolved", "90 days", "60 days", "45 days")
	recentlyResolved := insert("Resolved", "90 days", "60 days", "10 days")
	newMessageSince := insert("Closed", "90 days", "5 days", "45 days")
	openOld := insert("Open", "90 days", "60 days", "60 days")
	taggedExpired := insert("Closed", "90 days", "60 days", "45 days")
	var tagID int
	if err := db.Get(&tagID, `INSERT INTO tags (name) VALUES ('payment') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversation_tags (tag_id, conversation_id) VALUES ($1, $2)`, tagID, taggedExpired); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO conversation_messages (type, status, conversation_id, sender_id, sender_type, content, created_at) VALUES ('incoming', 'received', $1, $2, 'contact', 'Me too', NOW() - '60 days'::interval)`, openOld, writerContact); err != nil {
		t.Fatal(err)
	}

	lo := logf.New(logf.Opts{})
	m := &Manager{lo: &lo, db: db}
	if err := dbutil.ScanSQLFile("queries.sql", &m.q, db, efs); err != nil {
		t.Fatal(err)
	}

	policy := RetentionPolicy{Days: 30, DeleteContacts: true, Tags: []string{"payment"}}
	// The preview counts what a run would delete, without touching anything.
	if count, err := m.CountExpiredConversations(context.Background(), policy); err != nil || count != 4 {
		t.Fatalf("count = %d, %v; want 4, nil", count, err)
	}
	// The first run with a policy is a dry run; it logs and deletes nothing.
	var runner retentionRunner
	runner.tick(context.Background(), m, policy)
	if count, err := m.CountExpiredConversations(context.Background(), policy); err != nil || count != 4 {
		t.Fatalf("count after dry run = %d, %v; want 4, nil", count, err)
	}
	// Retention switched off resets the runner, so the next policy gets a dry run again.
	runner.tick(context.Background(), m, RetentionPolicy{})
	runner.tick(context.Background(), m, policy)
	if count, err := m.CountExpiredConversations(context.Background(), policy); err != nil || count != 4 {
		t.Fatalf("count after second dry run = %d, %v; want 4, nil", count, err)
	}

	// The tagged conversation is kept while its own period is 0.
	deleted, deletedContacts, err := m.DeleteExpiredConversations(context.Background(), policy)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 4 || deletedContacts != 1 {
		t.Fatalf("deleted = %d conversations, %d contacts; want 4, 1", deleted, deletedContacts)
	}
	for id, want := range map[int]bool{lonelyContact: false, writerContact: true, contactID: true} {
		var exists bool
		if err := db.Get(&exists, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, id); err != nil {
			t.Fatal(err)
		}
		if exists != want {
			t.Errorf("contact %d exists = %v, want %v", id, exists, want)
		}
	}
	for id, want := range map[int]bool{lonelyExpired: false, writerExpired: false, expiredClosed: false, expiredResolved: false, messageInFlight: true, recentlyResolved: true, newMessageSince: true, openOld: true, taggedExpired: true} {
		var exists bool
		if err := db.Get(&exists, `SELECT EXISTS (SELECT 1 FROM conversations WHERE id = $1)`, id); err != nil {
			t.Fatal(err)
		}
		if exists != want {
			t.Errorf("conversation %d exists = %v, want %v", id, exists, want)
		}
	}
	// The deleted messages leave their Message-IDs behind, so the IMAP scan won't import the mails again.
	var tombstones int
	if err := db.Get(&tombstones, `SELECT COUNT(*) FROM deleted_message_source_ids`); err != nil {
		t.Fatal(err)
	}
	if tombstones == 0 {
		t.Fatal("no tombstones left for deleted messages")
	}
	var messages int
	if err := db.Get(&messages, `SELECT COUNT(*) FROM conversation_messages WHERE conversation_id IN ($1, $2)`, expiredClosed, expiredResolved); err != nil {
		t.Fatal(err)
	}
	if messages != 0 {
		t.Fatalf("messages of deleted conversations = %d, want 0", messages)
	}

	// A second run finds nothing left; the tagged conversation also waits while its period hasn't passed.
	if deleted, _, err := m.DeleteExpiredConversations(context.Background(), RetentionPolicy{Days: 30, DeleteContacts: true, Tags: []string{"payment"}, TaggedDays: 3650}); err != nil || deleted != 0 {
		t.Fatalf("second run = %d, %v; want 0, nil", deleted, err)
	}
	// Once the tagged period has passed too, the tagged conversation goes.
	if deleted, _, err := m.DeleteExpiredConversations(context.Background(), RetentionPolicy{Days: 30, Tags: []string{"payment"}, TaggedDays: 40}); err != nil || deleted != 1 || exists("conversations", taggedExpired) {
		t.Fatalf("tagged run = %d, %v; want 1, nil and the tagged conversation gone", deleted, err)
	}

	// Without the contact option the conversation goes and the contact stays.
	keptContact := newContact("kept@example.com")
	insertFor(keptContact, "Closed", "90 days", "60 days", "45 days")
	if deleted, deletedContacts, err := m.DeleteExpiredConversations(context.Background(), RetentionPolicy{Days: 30}); err != nil || deleted != 1 || deletedContacts != 0 {
		t.Fatalf("run without contacts = %d, %d, %v; want 1, 0, nil", deleted, deletedContacts, err)
	}
	if !exists("users", keptContact) {
		t.Fatal("contact deleted although the option is off")
	}
}
