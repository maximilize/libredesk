package conversation

import (
	"encoding/json"
	"testing"

	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/zerodha/logf"
)

func TestDraftSuggestionIsSharedAndKeptApartFromOwnDrafts(t *testing.T) {
	db := testutil.NewDB(t, "conversation_draft_suggestion")
	var q queries
	if err := dbutil.ScanSQLFile("queries.sql", &q, db, efs); err != nil {
		t.Fatalf("preparing conversation queries: %v", err)
	}
	lo := logf.New(logf.Opts{Level: logf.FatalLevel})
	m := &Manager{q: q, lo: &lo}

	var contactID, agentID, aiID, inboxID, statusID, convID, otherConvID int
	mustScan := func(dest *int, query string, args ...any) {
		t.Helper()
		if err := db.QueryRow(query, args...).Scan(dest); err != nil {
			t.Fatalf("seeding (%s): %v", query, err)
		}
	}
	mustScan(&contactID, `INSERT INTO users (type, email, first_name) VALUES ('contact', 'suggestion-contact@example.test', 'Clara') RETURNING id`)
	mustScan(&agentID, `INSERT INTO users (type, email, first_name) VALUES ('agent', 'suggestion-agent@example.test', 'Agent') RETURNING id`)
	mustScan(&aiID, `INSERT INTO users (type, email, first_name, last_name) VALUES ('agent', 'suggestion-ai@example.test', 'AI', 'Prep') RETURNING id`)
	mustScan(&inboxID, `INSERT INTO inboxes (channel, name) VALUES ('email', 'suggestion') RETURNING id`)
	mustScan(&statusID, `SELECT id FROM conversation_statuses WHERE category = 'open' LIMIT 1`)
	mustScan(&convID, `INSERT INTO conversations (contact_id, inbox_id, status_id, subject) VALUES ($1, $2, $3, 'invoice') RETURNING id`, contactID, inboxID, statusID)
	mustScan(&otherConvID, `INSERT INTO conversations (contact_id, inbox_id, status_id, subject) VALUES ($1, $2, $3, 'other') RETURNING id`, contactID, inboxID, statusID)

	none, err := m.GetConversationDraftSuggestion(convID)
	if err != nil || none != nil {
		t.Fatalf("expected no suggestion before one is stored, got %+v, %v", none, err)
	}

	suggestionMeta := json.RawMessage(`{"suggestion": true, "source": "ai-prep"}`)
	if _, err := m.UpsertConversationDraft(convID, agentID, "reply", "<p>my own draft</p>", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("saving the agent's own draft: %v", err)
	}
	if _, err := m.UpsertConversationDraft(convID, aiID, "reply", "<p>first suggestion</p>", suggestionMeta); err != nil {
		t.Fatalf("saving a suggestion: %v", err)
	}
	if _, err := m.UpsertConversationDraft(convID, aiID, "reply", "<p>newer suggestion</p>", suggestionMeta); err != nil {
		t.Fatalf("replacing the suggestion: %v", err)
	}
	if _, err := m.UpsertConversationDraft(otherConvID, aiID, "reply", "<p>other conversation</p>", suggestionMeta); err != nil {
		t.Fatalf("saving a suggestion elsewhere: %v", err)
	}

	got, err := m.GetConversationDraftSuggestion(convID)
	if err != nil || got == nil {
		t.Fatalf("expected a suggestion, got %+v, %v", got, err)
	}
	if got.Content != "<p>newer suggestion</p>" || got.UserID != int64(aiID) || got.Author.FirstName != "AI" || got.Author.LastName != "Prep" {
		t.Fatalf("unexpected suggestion: %+v", got)
	}

	var rows int
	mustScan(&rows, `SELECT COUNT(*) FROM conversation_drafts WHERE conversation_id = $1 AND user_id = $2`, convID, aiID)
	if rows != 1 {
		t.Fatalf("a new suggestion from the same user must replace the old one, got %d rows", rows)
	}

	// The agent's draft list stays their own: no suggestions from others.
	own, err := m.GetAllUserDrafts(agentID)
	if err != nil || len(own) != 1 || own[0].Content != "<p>my own draft</p>" {
		t.Fatalf("agent drafts changed: %+v, %v", own, err)
	}

	if err := m.DeleteConversationDraftSuggestions(convID); err != nil {
		t.Fatalf("deleting suggestions: %v", err)
	}
	if got, _ := m.GetConversationDraftSuggestion(convID); got != nil {
		t.Fatalf("suggestion still there after delete: %+v", got)
	}
	if own, _ := m.GetAllUserDrafts(agentID); len(own) != 1 {
		t.Fatalf("deleting suggestions must keep the agent's own draft, got %+v", own)
	}
	if other, _ := m.GetConversationDraftSuggestion(otherConvID); other == nil {
		t.Fatal("deleting suggestions must not touch other conversations")
	}
}
