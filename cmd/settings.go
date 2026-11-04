package main

import (
	"encoding/json"
	"net/mail"
	"strconv"
	"strings"

	"github.com/abhinavxd/libredesk/internal/conversation"

	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/abhinavxd/libredesk/internal/httputil"
	"github.com/abhinavxd/libredesk/internal/setting/models"
	"github.com/abhinavxd/libredesk/internal/stringutil"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// handleGetGeneralSettings fetches general settings, this endpoint is not behind auth as it has no sensitive data and is required for the app to function.
func handleGetGeneralSettings(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
	)
	out, err := app.setting.GetByPrefix("app")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	// Unmarshal to set the app.update to the settings, so the frontend can show that an update is available.
	var settings map[string]interface{}
	if err := json.Unmarshal(out, &settings); err != nil {
		app.lo.Error("error unmarshalling settings", "err", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}
	// Set the app.update to the settings, adding `app` prefix to the key to match the settings structure in db.
	settings["app.update"] = app.update
	// Set app version.
	settings["app.version"] = versionString
	// Set restart required flag.
	settings["app.restart_required"] = app.restartRequired
	return r.SendEnvelope(settings)
}

// handleUpdateGeneralSettings updates general settings.
// Retention periods are 0 (off) or between minConversationRetentionDays and maxConversationRetentionDays,
// so a typo like 1 or 7 can't wipe recent conversations.
const (
	minConversationRetentionDays = 30
	maxConversationRetentionDays = 3650
)

// retentionEnabled reports whether scheduled deletion is switched on in config.toml.
func retentionEnabled() bool {
	return ko.Bool("conversation.retention_enabled")
}

// validRetentionPeriod accepts 0 or a value within the allowed range.
func validRetentionPeriod(days int) bool {
	return days == 0 || (days >= minConversationRetentionDays && days <= maxConversationRetentionDays)
}

// retentionRequested reports whether the settings ask for any scheduled deletion.
func retentionRequested(req models.General) bool {
	return req.ConversationRetentionDays != 0 || req.ConversationRetentionTaggedDays != 0 || len(req.ConversationRetentionTags) != 0 || req.ConversationRetentionDeleteContacts
}

// cleanTagNames trims the names, drops empty ones and duplicates, keeping the order.
func cleanTagNames(names []string) []string {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func handleUpdateGeneralSettings(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = models.General{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Trim whitespace from string fields.
	req.SiteName = strings.TrimSpace(req.SiteName)
	req.FaviconURL = strings.TrimSpace(req.FaviconURL)
	req.LogoURL = strings.TrimSpace(req.LogoURL)
	req.Timezone = strings.TrimSpace(req.Timezone)
	if req.Timezone != "" && !stringutil.IsValidTimezone(req.Timezone) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid timezone.", nil, envelope.InputError)
	}
	// Trim whitespace and trailing slash from root URL.
	req.RootURL = strings.TrimRight(strings.TrimSpace(req.RootURL), "/")
	if !httputil.IsValidHTTPURL(req.RootURL) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.general.rootURL.valid"), nil, envelope.InputError)
	}
	req.ConversationRetentionTags = cleanTagNames(req.ConversationRetentionTags)
	if !retentionEnabled() && retentionRequested(req) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.general.conversationRetention.disabled"), nil, envelope.InputError)
	}
	if !validRetentionPeriod(req.ConversationRetentionDays) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.general.conversationRetentionDays.valid"), nil, envelope.InputError)
	}
	if !validRetentionPeriod(req.ConversationRetentionTaggedDays) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.general.conversationRetentionTaggedDays.valid"), nil, envelope.InputError)
	}

	app.Lock()
	oldLang := ko.String("app.lang")
	oldRootURL := ko.String("app.root_url")
	app.Unlock()

	if err := app.setting.Update(req); err != nil {
		return sendErrorEnvelope(r, err)
	}
	// Reload the settings and templates.
	if err := reloadSettings(app); err != nil {
		app.lo.Error("error reloading settings", "error", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	// Check if language changed and reload i18n if needed.
	app.Lock()
	newLang := ko.String("app.lang")
	if oldLang != newLang {
		app.lo.Info("language changed, reloading i18n", "old_lang", oldLang, "new_lang", newLang)
		app.i18n = initI18n(app.fs)
		app.lo.Info("reloaded i18n", "old_lang", oldLang, "new_lang", newLang)
	}
	app.Unlock()

	if err := reloadTemplates(app); err != nil {
		app.lo.Error("error reloading templates", "error", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	if strings.TrimRight(oldRootURL, "/") != req.RootURL {
		go reconcileWhatsAppRootURL(app)
	}

	return r.SendEnvelope(true)
}

// handleGetEmailNotificationSettings fetches email notification settings.
func handleGetEmailNotificationSettings(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		notif = models.EmailNotification{}
	)

	out, err := app.setting.GetByPrefix("notification.email")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Unmarshal and filter out password.
	if err := json.Unmarshal(out, &notif); err != nil {
		app.lo.Error("error decoding notification settings", "error", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}
	if notif.Password != "" {
		notif.Password = strings.Repeat(stringutil.PasswordDummy, 10)
	}
	return r.SendEnvelope(notif)
}

// handleUpdateEmailNotificationSettings updates email notification settings.
func handleUpdateEmailNotificationSettings(r *fastglue.Request) error {
	var (
		app = r.Context.(*App)
		req = models.EmailNotification{}
		cur = models.EmailNotification{}
	)

	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.badRequest"), nil, envelope.InputError)
	}

	// Trim whitespace from string fields (Password intentionally NOT trimmed).
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.EmailAddress = strings.TrimSpace(req.EmailAddress)
	req.HelloHostname = strings.TrimSpace(req.HelloHostname)
	req.IdleTimeout = strings.TrimSpace(req.IdleTimeout)
	req.WaitTimeout = strings.TrimSpace(req.WaitTimeout)

	out, err := app.setting.GetByPrefix("notification.email")
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := json.Unmarshal(out, &cur); err != nil {
		app.lo.Error("error decoding notification settings", "error", err)
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	// Make sure it's a valid from email address.
	if _, err := mail.ParseAddress(req.EmailAddress); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("validation.invalidFromAddress"), nil, envelope.InputError)
	}

	// Retain current password if not changed.
	if req.Password == "" {
		req.Password = cur.Password
	}

	if err := app.setting.Update(req); err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Email notification settings require app restart to take effect.
	app.Lock()
	app.restartRequired = true
	app.Unlock()

	return r.SendEnvelope(true)
}

// handleGetRetentionPreview returns how many resolved conversations the given retention values would delete
// right now, so an admin sees the effect before saving. Query: days, tagged_days, tags (comma separated).
func handleGetRetentionPreview(r *fastglue.Request) error {
	var app = r.Context.(*App)
	if !retentionEnabled() {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.general.conversationRetention.disabled"), nil, envelope.InputError)
	}
	args := r.RequestCtx.QueryArgs()
	days, _ := strconv.Atoi(string(args.Peek("days")))
	taggedDays, _ := strconv.Atoi(string(args.Peek("tagged_days")))
	if !validRetentionPeriod(days) || !validRetentionPeriod(taggedDays) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("admin.general.conversationRetentionDays.valid"), nil, envelope.InputError)
	}
	policy := conversation.RetentionPolicy{Days: days, TaggedDays: taggedDays, Tags: cleanTagNames(strings.Split(string(args.Peek("tags")), ","))}
	count := 0
	if policy.Days > 0 {
		var err error
		if count, err = app.conversation.CountExpiredConversations(r.RequestCtx, policy); err != nil {
			return sendErrorEnvelope(r, err)
		}
	}
	return r.SendEnvelope(map[string]int{"count": count})
}
