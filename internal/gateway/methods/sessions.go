package methods

import (
	"context"
	"encoding/json"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	httpapi "github.com/nextlevelbuilder/goclaw/internal/http"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// SessionsMethods handles sessions.list, sessions.preview, sessions.patch, sessions.delete, sessions.reset.
type SessionsMethods struct {
	sessions store.SessionStore
	eventBus bus.EventPublisher
	cfg      *config.Config
}

func NewSessionsMethods(sess store.SessionStore, eventBus bus.EventPublisher, cfg *config.Config) *SessionsMethods {
	return &SessionsMethods{sessions: sess, eventBus: eventBus, cfg: cfg}
}

func (m *SessionsMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodSessionsList, m.handleList)
	router.Register(protocol.MethodSessionsPreview, m.handlePreview)
	router.Register(protocol.MethodSessionsPatch, m.handlePatch)
	router.Register(protocol.MethodSessionsDelete, m.handleDelete)
	router.Register(protocol.MethodSessionsReset, m.handleReset)
	router.Register(protocol.MethodSessionsCompact, m.handleCompact)
	router.Register(protocol.MethodSessionsRecap, m.handleRecap)
	router.Register(protocol.MethodSessionsExport, m.handleExport)
	router.Register(protocol.MethodSessionsFork, m.handleFork)
	router.Register(protocol.MethodSessionsRewind, m.handleRewind)
	router.Register(protocol.MethodSessionsGoalSet, m.handleGoalSet)
	router.Register(protocol.MethodSessionsGoalStatus, m.handleGoalStatus)
}

type sessionsListParams struct {
	AgentID string `json:"agentId"`
	Channel string `json:"channel"` // optional: filter by channel prefix ("ws", "telegram")
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func (m *SessionsMethods) handleList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params sessionsListParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Limit <= 0 {
		params.Limit = 20
	}

	opts := store.SessionListOpts{
		AgentID:  params.AgentID,
		Channel:  params.Channel,
		Limit:    params.Limit,
		Offset:   params.Offset,
		TenantID: store.TenantIDFromContext(ctx),
	}
	// Role-based filtering: admins/owners see all sessions; regular users see only their own.
	// Tenant scope is always applied above — admin sees all sessions within the tenant.
	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		opts.UserID = client.UserID()
	}

	result := m.sessions.ListPagedRich(ctx, opts)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"sessions": result.Sessions,
		"total":    result.Total,
		"limit":    params.Limit,
		"offset":   params.Offset,
	}))
}

type sessionKeyParams struct {
	Key string `json:"key"`
}

func (m *SessionsMethods) handlePreview(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionKeyParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	history := m.sessions.GetHistory(ctx, params.Key)
	summary := m.sessions.GetSummary(ctx, params.Key)

	// Sign file URLs before delivery — sessions store clean paths.
	secret := httpapi.FileSigningKey()
	for i := range history {
		history[i].Content = httpapi.SignFileURLs(history[i].Content, secret)
		for j := range history[i].MediaRefs {
			history[i].MediaRefs[j].Path = httpapi.SignMediaPath(history[i].MediaRefs[j].Path, secret)
		}
	}
	summary = httpapi.SignFileURLs(summary, secret)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"key":      params.Key,
		"messages": history,
		"summary":  summary,
	}))
}

// handlePatch updates session metadata fields.
// Matching TS sessions.patch (src/gateway/server-methods/sessions.ts:237-287).
func (m *SessionsMethods) handlePatch(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params struct {
		Key      string            `json:"key"`
		Label    *string           `json:"label,omitempty"`
		Model    *string           `json:"model,omitempty"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if params.Key == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "key")))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	// Apply label patch
	if params.Label != nil {
		m.sessions.SetLabel(ctx, params.Key, *params.Label)
	}

	// Apply model patch
	if params.Model != nil {
		m.sessions.UpdateMetadata(ctx, params.Key, *params.Model, "", "")
	}

	// Apply metadata patch
	if len(params.Metadata) > 0 {
		m.sessions.SetSessionMetadata(ctx, params.Key, params.Metadata)
	}

	// Save changes to DB
	m.sessions.Save(ctx, params.Key)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok":  true,
		"key": params.Key,
	}))
	emitAudit(m.eventBus, client, "session.patched", "session", params.Key)
}

func (m *SessionsMethods) handleDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionKeyParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	if err := m.sessions.Delete(ctx, params.Key); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok": true,
	}))
	emitAudit(m.eventBus, client, "session.deleted", "session", params.Key)
}

func (m *SessionsMethods) handleReset(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionKeyParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	m.sessions.Reset(ctx, params.Key)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok": true,
	}))
	emitAudit(m.eventBus, client, "session.reset", "session", params.Key)
}

type sessionCompactParams struct {
	Key      string `json:"key"`
	KeepLast int    `json:"keepLast,omitempty"` // default 4
}

// handleCompact truncates session history to the last N messages.
// Issue 958: Manual session compaction API (truncate-only, no LLM summarization).
func (m *SessionsMethods) handleCompact(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionCompactParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if params.Key == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "key is required"))
		return
	}

	keepLast := params.KeepLast
	if keepLast <= 0 {
		keepLast = 4 // default: keep last 2 exchanges
	}

	// Auth check
	sess := m.sessions.Get(ctx, params.Key)
	if sess == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
		return
	}
	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	history := m.sessions.GetHistory(ctx, params.Key)
	originalLen := len(history)
	if originalLen < 6 {
		client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
			"ok":      true,
			"message": "session too short to compact",
			"kept":    originalLen,
		}))
		return
	}

	// Truncate history to last N messages
	m.sessions.TruncateHistory(ctx, params.Key, keepLast)
	m.sessions.IncrementCompaction(ctx, params.Key)
	m.sessions.Save(ctx, params.Key)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok":       true,
		"original": originalLen,
		"kept":     keepLast,
	}))
	emitAudit(m.eventBus, client, "session.compacted", "session", params.Key)
}

// --- sessions.recap ---

type sessionsRecapParams struct {
	SessionKey string `json:"sessionKey"`
}

func (m *SessionsMethods) handleRecap(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionsRecapParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.SessionKey == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "sessionKey")))
		return
	}
	if !requireSessionOwner(ctx, m.sessions, m.cfg, client, req.ID, params.SessionKey) {
		return
	}

	sess := m.sessions.Get(ctx, params.SessionKey)
	if sess == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.SessionKey)))
		return
	}

	payload := map[string]any{
		"sessionKey":   sess.Key,
		"label":        sess.Label,
		"messageCount": len(sess.Messages),
		"totalTokens":  sess.InputTokens + sess.OutputTokens,
		"created":      sess.Created,
		"updated":      sess.Updated,
		"model":        sess.Model,
		"provider":     sess.Provider,
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, payload))
}

// --- sessions.export ---

func (m *SessionsMethods) handleExport(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionsRecapParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.SessionKey == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "sessionKey")))
		return
	}
	if !requireSessionOwner(ctx, m.sessions, m.cfg, client, req.ID, params.SessionKey) {
		return
	}

	sess := m.sessions.Get(ctx, params.SessionKey)
	if sess == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.SessionKey)))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"session": sess,
	}))
}

// --- sessions.fork ---

type sessionsForkParams struct {
	SessionKey string `json:"sessionKey"`
	NewLabel   string `json:"label,omitempty"`
}

func (m *SessionsMethods) handleFork(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionsForkParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.SessionKey == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "sessionKey")))
		return
	}
	if !requireSessionOwner(ctx, m.sessions, m.cfg, client, req.ID, params.SessionKey) {
		return
	}

	sess := m.sessions.Get(ctx, params.SessionKey)
	if sess == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.SessionKey)))
		return
	}

	// Derive new session key: append -fork-<timestamp>
	newKey := params.SessionKey + "-fork"
	newLabel := params.NewLabel
	if newLabel == "" {
		newLabel = (sess.Label + " (fork)")
	}

	// Create fork via GetOrCreate + SetHistory
	forkSess := m.sessions.GetOrCreate(ctx, newKey)
	forkSess.Messages = make([]providers.Message, len(sess.Messages))
	copy(forkSess.Messages, sess.Messages)
	m.sessions.SetLabel(ctx, newKey, newLabel)
	m.sessions.SetHistory(ctx, newKey, forkSess.Messages)
	m.sessions.Save(ctx, newKey)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"sessionKey": newKey,
		"label":      newLabel,
	}))
}

// --- sessions.rewind ---

type sessionsRewindParams struct {
	SessionKey string `json:"sessionKey"`
	Turns      int    `json:"turns"` // number of turns to undo (default 1)
}

func (m *SessionsMethods) handleRewind(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionsRewindParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.SessionKey == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "sessionKey")))
		return
	}
	if params.Turns <= 0 {
		params.Turns = 1
	}
	if !requireSessionOwner(ctx, m.sessions, m.cfg, client, req.ID, params.SessionKey) {
		return
	}

	sess := m.sessions.Get(ctx, params.SessionKey)
	if sess == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.SessionKey)))
		return
	}

	// Remove last N user+assistant pairs (turns)
	msgs := sess.Messages
	removed := 0
	for i := 0; i < params.Turns && len(msgs) > 0; i++ {
		// Find last user message
		lastUser := -1
		for j := len(msgs) - 1; j >= 0; j-- {
			if msgs[j].Role == "user" {
				lastUser = j
				break
			}
		}
		if lastUser < 0 {
			break
		}
		removed += len(msgs) - lastUser
		msgs = msgs[:lastUser]
	}

	sess.Messages = msgs
	m.sessions.TruncateHistory(ctx, params.SessionKey, len(msgs))
	m.sessions.Save(ctx, params.SessionKey)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"sessionKey":     params.SessionKey,
		"removedTurns":   params.Turns,
		"removedMessages": removed,
		"remainingMessages": len(msgs),
	}))
}

// --- sessions.goal ---

type sessionsGoalParams struct {
	SessionKey string `json:"sessionKey"`
	Goal       string `json:"goal,omitempty"` // empty = get status only
}

func (m *SessionsMethods) handleGoalSet(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionsGoalParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.SessionKey == "" || params.Goal == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "sessionKey + goal")))
		return
	}

	// Store goal in session metadata
	m.sessions.SetSessionMetadata(ctx, params.SessionKey, map[string]string{
		"goal":        params.Goal,
		"goal_status": "active",
	})

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"sessionKey": params.SessionKey,
		"goal":       params.Goal,
		"status":     "active",
	}))
}

func (m *SessionsMethods) handleGoalStatus(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionsGoalParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.SessionKey == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "sessionKey")))
		return
	}

	sess := m.sessions.Get(ctx, params.SessionKey)
	if sess == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.SessionKey)))
		return
	}

	goal := ""
	goalStatus := ""
	if sess.Metadata != nil {
		goal = sess.Metadata["goal"]
		goalStatus = sess.Metadata["goal_status"]
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"sessionKey": params.SessionKey,
		"goal":       goal,
		"status":     goalStatus,
	}))
}
