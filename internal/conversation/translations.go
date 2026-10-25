package conversation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
)

// languageCodePattern accepts short language tags such as "de", "fr" or "pt-BR".
var languageCodePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)

// maxTranslationLength caps a stored translation, matching the size of a long email body.
const maxTranslationLength = 100000

// NormalizeLanguageCode trims and lowercases the primary subtag, returning "" for anything that isn't a language tag.
func NormalizeLanguageCode(code string) string {
	code = strings.TrimSpace(code)
	if primary, rest, found := strings.Cut(code, "-"); found {
		code = strings.ToLower(primary) + "-" + rest
	} else {
		code = strings.ToLower(code)
	}
	if !languageCodePattern.MatchString(code) {
		return ""
	}
	return code
}

// SetMessageTranslations merges translations into the message's meta.translations, records its
// language when one is given, and pushes the new meta to clients viewing the conversation.
func (m *Manager) SetMessageTranslations(conversationUUID, messageUUID, language string, translations map[string]models.MessageTranslation) (json.RawMessage, error) {
	badRequest := envelope.NewError(envelope.InputError, m.i18n.T("globals.messages.badRequest"), nil)
	if language != "" {
		if language = NormalizeLanguageCode(language); language == "" {
			return nil, badRequest
		}
	}

	cleaned := make(map[string]models.MessageTranslation, len(translations))
	for code, t := range translations {
		code = NormalizeLanguageCode(code)
		t.Text = strings.TrimSpace(t.Text)
		if code == "" || t.Text == "" {
			return nil, badRequest
		}
		if len(t.Text) > maxTranslationLength {
			return nil, envelope.NewError(envelope.InputError, m.i18n.Ts("globals.messages.maxLength", "max", "100000"), nil)
		}
		if t.Source != models.TranslationSourceOriginal {
			t.Source = models.TranslationSourceAI
		}
		if t.From != "" {
			t.From = NormalizeLanguageCode(t.From)
		}
		if t.CreatedAt.IsZero() {
			t.CreatedAt = time.Now().UTC()
		}
		cleaned[code] = t
	}
	if len(cleaned) == 0 && language == "" {
		return nil, badRequest
	}

	b, err := json.Marshal(cleaned)
	if err != nil {
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	var meta json.RawMessage
	if err := m.q.SetMessageTranslations.Get(&meta, messageUUID, conversationUUID, language, b); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, envelope.NewError(envelope.NotFoundError, m.i18n.T("globals.messages.notFound"), nil)
		}
		m.lo.Error("error storing message translations", "message_uuid", messageUUID, "error", err)
		return nil, envelope.NewError(envelope.GeneralError, m.i18n.T("globals.messages.somethingWentWrong"), nil)
	}
	m.BroadcastMessageUpdate(conversationUUID, messageUUID, map[string]any{"meta": stripCSATUUID(meta)})
	return meta, nil
}

// TranslateMessage returns the message translated into language: from meta.translations when it was
// translated before, as is when the message is already in that language, else through translate,
// storing the result. cached reports whether translate was skipped.
func (m *Manager) TranslateMessage(conversationUUID, messageUUID, language string, translate func(text string) (string, error)) (models.MessageTranslation, bool, error) {
	badRequest := envelope.NewError(envelope.InputError, m.i18n.T("globals.messages.badRequest"), nil)
	if language = NormalizeLanguageCode(language); language == "" {
		return models.MessageTranslation{}, false, badRequest
	}
	message, err := m.GetMessage(messageUUID)
	if err != nil {
		return models.MessageTranslation{}, false, err
	}
	if message.ConversationUUID != conversationUUID {
		return models.MessageTranslation{}, false, envelope.NewError(envelope.NotFoundError, m.i18n.T("globals.messages.notFound"), nil)
	}
	if message.Type == models.MessageActivity || strings.TrimSpace(message.TextContent) == "" {
		return models.MessageTranslation{}, false, badRequest
	}
	if t, ok := message.Translation(language); ok {
		return t, true, nil
	}
	if message.Language() == language {
		return models.MessageTranslation{Text: message.TextContent, From: language, Source: models.TranslationSourceOriginal}, true, nil
	}

	text, err := translate(message.TextContent)
	if err != nil {
		return models.MessageTranslation{}, false, err
	}
	meta, err := m.SetMessageTranslations(conversationUUID, messageUUID, "", map[string]models.MessageTranslation{
		language: {Text: text, From: message.Language(), Source: models.TranslationSourceAI},
	})
	if err != nil {
		return models.MessageTranslation{}, false, err
	}
	stored := models.Message{Meta: meta}
	t, _ := stored.Translation(language)
	return t, false, nil
}
