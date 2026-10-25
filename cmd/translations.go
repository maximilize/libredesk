package main

import (
	"encoding/json"
	"strings"
	"time"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	"github.com/abhinavxd/libredesk/internal/conversation"
	cmodels "github.com/abhinavxd/libredesk/internal/conversation/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/zerodha/fastglue"
)

// translationLanguageSetting holds the team's working language; empty turns message translations off.
const translationLanguageSetting = "ai.translation_language"

type translationLanguageReq struct {
	Language string `json:"language"`
}

type messageTranslationsReq struct {
	LanguageDetected string                                `json:"language_detected"`
	Translations     map[string]cmodels.MessageTranslation `json:"translations"`
}

type translateMessageReq struct {
	Language string `json:"language"`
}

type aiTranslateReq struct {
	Content  string `json:"content"`
	Language string `json:"language"`
}

// translationOriginalReq is the text an agent wrote before translating their reply into the customer's language.
type translationOriginalReq struct {
	Language     string `json:"language"`
	Text         string `json:"text"`
	SentLanguage string `json:"sent_language"`
}

// teamTranslationLanguage returns the configured working language, or "" when translations are off.
func teamTranslationLanguage(app *App) string {
	b, err := app.setting.Get(translationLanguageSetting)
	if err != nil {
		return ""
	}
	var language string
	_ = json.Unmarshal(b, &language)
	return conversation.NormalizeLanguageCode(language)
}

// handleGetTranslationLanguage returns the team's working language for message translations.
func handleGetTranslationLanguage(r *fastglue.Request) error {
	app := r.Context.(*App)
	return r.SendEnvelope(map[string]string{"language": teamTranslationLanguage(app)})
}

// handleUpdateTranslationLanguage sets the team's working language; an empty language turns translations off.
func handleUpdateTranslationLanguage(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req translationLanguageReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	language := ""
	if strings.TrimSpace(req.Language) != "" {
		if language = conversation.NormalizeLanguageCode(req.Language); language == "" {
			return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.badRequest"), nil))
		}
	}
	if err := app.setting.Update(map[string]any{translationLanguageSetting: language}); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(map[string]string{"language": language})
}

// handleSetMessageTranslations stores translations on a message, for services that translate incoming mail.
func handleSetMessageTranslations(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   messageTranslationsReq
	)
	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := enforceConversationAccess(app, cuuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	meta, err := app.conversation.SetMessageTranslations(cuuid, uuid, req.LanguageDetected, req.Translations)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(map[string]any{"meta": meta})
}

// handleTranslateMessage returns a message translated into the requested language, from meta.translations
// when it was translated before, else through the configured AI provider, storing the result.
func handleTranslateMessage(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		cuuid = r.RequestCtx.UserValue("cuuid").(string)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		req   translateMessageReq
	)
	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	if _, err := enforceConversationAccess(app, cuuid, user); err != nil {
		return sendErrorEnvelope(r, err)
	}
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	language := conversation.NormalizeLanguageCode(req.Language)
	t, cached, err := app.conversation.TranslateMessage(cuuid, uuid, language, func(text string) (string, error) {
		return app.ai.Translate(r.RequestCtx, text, language)
	})
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(map[string]any{"language": language, "translation": t, "cached": cached})
}

// handleAITranslate translates the reply editor's HTML into a language, keeping the markup.
func handleAITranslate(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req aiTranslateReq
	)
	if err := r.Decode(&req, "json"); err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("errors.parsingRequest"), nil))
	}
	language := conversation.NormalizeLanguageCode(req.Language)
	if language == "" || strings.TrimSpace(req.Content) == "" {
		return sendErrorEnvelope(r, envelope.NewError(envelope.InputError, app.i18n.T("globals.messages.badRequest"), nil))
	}
	out, err := app.ai.TranslateHTML(r.RequestCtx, req.Content, language)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(out)
}

// originalTranslationMeta turns the agent's pre-translation text into reply meta, or nil when it is unusable.
func originalTranslationMeta(o *translationOriginalReq) map[string]any {
	if o == nil {
		return nil
	}
	language := conversation.NormalizeLanguageCode(o.Language)
	text := strings.TrimSpace(o.Text)
	if language == "" || text == "" {
		return nil
	}
	sent := conversation.NormalizeLanguageCode(o.SentLanguage)
	meta := map[string]any{
		"translations": map[string]cmodels.MessageTranslation{
			language: {Text: text, From: sent, Source: cmodels.TranslationSourceOriginal, CreatedAt: time.Now().UTC()},
		},
	}
	if sent != "" {
		meta["language"] = sent
	}
	return meta
}
