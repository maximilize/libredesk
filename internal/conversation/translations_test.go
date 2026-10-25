package conversation

import (
	"encoding/json"
	"testing"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/dbutil"
	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/abhinavxd/libredesk/internal/ws"
	"github.com/zerodha/logf"
)

func TestSetMessageTranslations(t *testing.T) {
	db := testutil.NewDB(t, "message_translations")
	var contactID, inboxID, conversationID int
	var conversationUUID, otherUUID string
	if err := db.Get(&contactID, `INSERT INTO users (type, email, first_name, last_name) VALUES ('contact', 'client@example.fr', 'Client', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&inboxID, `INSERT INTO inboxes (name, channel) VALUES ('Mail', 'email') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	insertConversation := func() (int, string) {
		var id int
		var uuid string
		if err := db.QueryRow(`INSERT INTO conversations (contact_id, inbox_id, status_id) VALUES ($1, $2, (SELECT id FROM conversation_statuses LIMIT 1)) RETURNING id, uuid`, contactID, inboxID).Scan(&id, &uuid); err != nil {
			t.Fatal(err)
		}
		return id, uuid
	}
	conversationID, conversationUUID = insertConversation()
	_, otherUUID = insertConversation()
	insertMessage := func(msgType, meta string) string {
		var uuid string
		if err := db.Get(&uuid, `INSERT INTO conversation_messages (type, status, conversation_id, sender_id, sender_type, content, text_content, content_type, meta) VALUES ($1, 'received', $2, $3, 'contact', 'Bonjour', 'Bonjour', 'text', $4) RETURNING uuid`, msgType, conversationID, contactID, meta); err != nil {
			t.Fatal(err)
		}
		return uuid
	}

	lo := logf.New(logf.Opts{})
	m := &Manager{lo: &lo, i18n: testutil.NewI18n(t), wsHub: ws.NewHub(&lo, nil)}
	if err := dbutil.ScanSQLFile("queries.sql", &m.q, db, efs); err != nil {
		t.Fatal(err)
	}

	msg := insertMessage("incoming", `{"echo_id": "e1", "translations": {"en": {"text": "Hello", "source": "ai"}}}`)
	meta, err := m.SetMessageTranslations(conversationUUID, msg, "FR", map[string]models.MessageTranslation{
		"DE": {Text: "  Guten Tag  ", From: "fr"},
	})
	if err != nil {
		t.Fatalf("set translations: %v", err)
	}
	stored := models.Message{Meta: meta}
	de, ok := stored.Translation("de")
	if !ok || de.Text != "Guten Tag" || de.Source != models.TranslationSourceAI || de.From != "fr" || de.CreatedAt.IsZero() {
		t.Fatalf("de translation = %+v, %v; want trimmed AI translation from fr with a timestamp", de, ok)
	}
	if en, ok := stored.Translation("en"); !ok || en.Text != "Hello" {
		t.Fatalf("existing en translation = %+v, %v; want it kept", en, ok)
	}
	if stored.Language() != "fr" {
		t.Fatalf("language = %q, want fr", stored.Language())
	}
	var rest map[string]any
	if err := json.Unmarshal(meta, &rest); err != nil || rest["echo_id"] != "e1" {
		t.Fatalf("other meta keys = %v, %v; want echo_id kept", rest, err)
	}

	// A later translation of the same language replaces it; an empty language leaves meta.language alone.
	meta, err = m.SetMessageTranslations(conversationUUID, msg, "", map[string]models.MessageTranslation{
		"de": {Text: "Hallo", Source: models.TranslationSourceOriginal},
	})
	if err != nil {
		t.Fatal(err)
	}
	stored = models.Message{Meta: meta}
	if de, _ := stored.Translation("de"); de.Text != "Hallo" || de.Source != models.TranslationSourceOriginal {
		t.Fatalf("replaced de = %+v; want the original text", de)
	}
	if stored.Language() != "fr" {
		t.Fatalf("language after update = %q, want fr kept", stored.Language())
	}

	for name, tc := range map[string]struct {
		conversation, message, language string
		translations                    map[string]models.MessageTranslation
	}{
		"other conversation": {otherUUID, msg, "", map[string]models.MessageTranslation{"de": {Text: "x"}}},
		"activity message":   {conversationUUID, insertMessage("activity", `{}`), "", map[string]models.MessageTranslation{"de": {Text: "x"}}},
		"bad language":       {conversationUUID, msg, "french", map[string]models.MessageTranslation{"de": {Text: "x"}}},
		"bad key":            {conversationUUID, msg, "", map[string]models.MessageTranslation{"deutsch": {Text: "x"}}},
		"empty text":         {conversationUUID, msg, "", map[string]models.MessageTranslation{"de": {Text: "  "}}},
		"nothing to store":   {conversationUUID, msg, "", nil},
	} {
		if _, err := m.SetMessageTranslations(tc.conversation, tc.message, tc.language, tc.translations); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestNormalizeLanguageCode(t *testing.T) {
	for in, want := range map[string]string{
		"de": "de", " FR ": "fr", "pt-BR": "pt-BR", "PT-br": "pt-br", "zh-Hant": "zh-Hant",
		"": "", "german": "", "d": "", "de_DE": "", "de-": "", "<b>": "",
	} {
		if got := NormalizeLanguageCode(in); got != want {
			t.Errorf("NormalizeLanguageCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTranslateMessageCachesResult(t *testing.T) {
	db := testutil.NewDB(t, "translate_message_cache")
	var contactID, inboxID, conversationID int
	var conversationUUID string
	if err := db.Get(&contactID, `INSERT INTO users (type, email, first_name, last_name) VALUES ('contact', 'cliente@example.it', 'Cliente', '') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&inboxID, `INSERT INTO inboxes (name, channel) VALUES ('Mail', 'email') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO conversations (contact_id, inbox_id, status_id) VALUES ($1, $2, (SELECT id FROM conversation_statuses LIMIT 1)) RETURNING id, uuid`, contactID, inboxID).Scan(&conversationID, &conversationUUID); err != nil {
		t.Fatal(err)
	}
	insertMessage := func(text, meta string) string {
		var uuid string
		if err := db.Get(&uuid, `INSERT INTO conversation_messages (type, status, conversation_id, sender_id, sender_type, content, text_content, content_type, meta) VALUES ('incoming', 'received', $1, $2, 'contact', $3, $3, 'text', $4) RETURNING uuid`, conversationID, contactID, text, meta); err != nil {
			t.Fatal(err)
		}
		return uuid
	}

	lo := logf.New(logf.Opts{})
	m := &Manager{lo: &lo, i18n: testutil.NewI18n(t), wsHub: ws.NewHub(&lo, nil)}
	if err := dbutil.ScanSQLFile("queries.sql", &m.q, db, efs); err != nil {
		t.Fatal(err)
	}

	calls := 0
	translate := func(text string) (string, error) {
		calls++
		return "Wo ist meine Zahlung?", nil
	}

	msg := insertMessage("Dov'è il mio pagamento?", `{"language": "it"}`)
	first, cached, err := m.TranslateMessage(conversationUUID, msg, "de", translate)
	if err != nil || cached || first.Text != "Wo ist meine Zahlung?" || first.From != "it" || first.Source != models.TranslationSourceAI {
		t.Fatalf("first translation = %+v, cached %v, %v; want a fresh AI translation from it", first, cached, err)
	}
	second, cached, err := m.TranslateMessage(conversationUUID, msg, "de", translate)
	if err != nil || !cached || second.Text != first.Text {
		t.Fatalf("second translation = %+v, cached %v, %v; want the stored one", second, cached, err)
	}
	if calls != 1 {
		t.Fatalf("translator calls = %d, want 1", calls)
	}

	// A message already in the target language comes back as is, without a translator call.
	same := insertMessage("Wo ist meine Zahlung?", `{"language": "de"}`)
	if got, cached, err := m.TranslateMessage(conversationUUID, same, "de", translate); err != nil || !cached || got.Text != "Wo ist meine Zahlung?" || calls != 1 {
		t.Fatalf("same-language translation = %+v, cached %v, %v, calls %d", got, cached, err, calls)
	}

	if _, _, err := m.TranslateMessage("00000000-0000-0000-0000-000000000000", msg, "de", translate); err == nil {
		t.Fatal("translating through another conversation's UUID must fail")
	}
}
