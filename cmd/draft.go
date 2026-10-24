package main

import (
	"encoding/json"
	"strings"

	amodels "github.com/abhinavxd/libredesk/internal/auth/models"
	"github.com/abhinavxd/libredesk/internal/envelope"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

const maxMetaSize = 32 * 1024 // 32KB

type draftReq struct {
	Type    string          `json:"type"`
	Content string          `json:"content"`
	Meta    json.RawMessage `json:"meta"`
}

// handleUpsertConversationDraft saves or updates a draft for a conversation.
func handleUpsertConversationDraft(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		req   = draftReq{}
	)

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	// Check access to conversation.
	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding draft request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	if req.Type == "" {
		req.Type = "reply"
	}
	if !isValidDraftType(req.Type) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	if len(req.Meta) > maxMetaSize {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	// Validate content is not empty
	if strings.TrimSpace(req.Content) == "" && (len(req.Meta) == 0 || string(req.Meta) == "{}") {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("globals.messages.somethingWentWrong"), nil, envelope.InputError)
	}

	draft, err := app.conversation.UpsertConversationDraft(conv.ID, user.ID, req.Type, req.Content, req.Meta)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(draft)
}

// handleGetAllDrafts retrieves all drafts for the current user.
func handleGetAllDrafts(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
	)

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	drafts, err := app.conversation.GetAllUserDrafts(user.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(drafts)
}

// handleDeleteConversationDraft deletes a draft for a conversation.
func handleDeleteConversationDraft(r *fastglue.Request) error {
	var (
		app       = r.Context.(*App)
		auser     = r.RequestCtx.UserValue("user").(amodels.User)
		uuid      = r.RequestCtx.UserValue("uuid").(string)
		draftType = string(r.RequestCtx.QueryArgs().Peek("type"))
	)

	if draftType != "" && !isValidDraftType(draftType) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := app.conversation.DeleteConversationDraft(0, uuid, user.ID, draftType); err != nil {
		return sendErrorEnvelope(r, err)
	}

	return r.SendEnvelope(true)
}

func isValidDraftType(t string) bool {
	return t == "reply" || t == "private_note"
}

type draftSuggestionReq struct {
	Content string          `json:"content"`
	Meta    json.RawMessage `json:"meta"`
}

// handlePutDraftSuggestion stores the caller's reply suggestion for a conversation. Agents without a draft of their own see it prefilled in the reply box.
func handlePutDraftSuggestion(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
		req   = draftSuggestionReq{}
	)

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := r.Decode(&req, "json"); err != nil {
		app.lo.Error("error decoding draft suggestion request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}
	if strings.TrimSpace(req.Content) == "" || len(req.Meta) > maxMetaSize {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
	}

	meta := map[string]any{}
	if len(req.Meta) > 0 && string(req.Meta) != "null" {
		if err := json.Unmarshal(req.Meta, &meta); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, app.i18n.T("errors.parsingRequest"), nil, envelope.InputError)
		}
	}
	meta["suggestion"] = true
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return sendErrorEnvelope(r, envelope.NewError(envelope.GeneralError, app.i18n.T("globals.messages.somethingWentWrong"), nil))
	}

	draft, err := app.conversation.UpsertConversationDraft(conv.ID, user.ID, "reply", req.Content, metaJSON)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(draft)
}

// handleGetDraftSuggestion returns the newest reply suggestion for a conversation, or null if there is none.
func handleGetDraftSuggestion(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
	)

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	suggestion, err := app.conversation.GetConversationDraftSuggestion(conv.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(suggestion)
}

// handleDeleteDraftSuggestion discards every reply suggestion for a conversation.
func handleDeleteDraftSuggestion(r *fastglue.Request) error {
	var (
		app   = r.Context.(*App)
		auser = r.RequestCtx.UserValue("user").(amodels.User)
		uuid  = r.RequestCtx.UserValue("uuid").(string)
	)

	user, err := app.user.GetAgentCachedOrLoad(auser.ID)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}
	conv, err := enforceConversationAccess(app, uuid, user)
	if err != nil {
		return sendErrorEnvelope(r, err)
	}

	if err := app.conversation.DeleteConversationDraftSuggestions(conv.ID); err != nil {
		return sendErrorEnvelope(r, err)
	}
	return r.SendEnvelope(true)
}
