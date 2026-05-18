package methods

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// remoteTokens stores active remote session access tokens.
// Token → sessionKey + userID + expires.
type remoteToken struct {
	SessionKey string
	UserID     string
	ExpiresAt  time.Time
}

var (
	remoteTokens   = map[string]*remoteToken{}
	remoteTokensMu sync.RWMutex
)

func init() {
	go func() {
		for range time.Tick(5 * time.Minute) {
			remoteTokensMu.Lock()
			now := time.Now()
			for k, v := range remoteTokens {
				if now.After(v.ExpiresAt) {
					delete(remoteTokens, k)
				}
			}
			remoteTokensMu.Unlock()
		}
	}()
}

// RemoteMethods handles sessions.remote.token.create and sessions.remote.attach.
type RemoteMethods struct{}

func NewRemoteMethods() *RemoteMethods { return &RemoteMethods{} }

func (m *RemoteMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodSessionsRemoteToken, m.handleTokenCreate)
	router.Register(protocol.MethodSessionsRemoteAttach, m.handleAttach)
}

type remoteTokenParams struct {
	SessionKey string `json:"sessionKey"`
}

func (m *RemoteMethods) handleTokenCreate(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params remoteTokenParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.SessionKey == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "sessionKey")))
		return
	}

	// Generate 16-byte token (32 hex chars)
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "failed to generate token"))
		return
	}
	token := hex.EncodeToString(b)

	remoteTokensMu.Lock()
	remoteTokens[token] = &remoteToken{
		SessionKey: params.SessionKey,
		UserID:     client.UserID(),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}
	remoteTokensMu.Unlock()

	slog.Info("remote.token.created",
		"sessionKey", params.SessionKey,
		"userId", client.UserID(),
		"expiresIn", "30m",
	)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"token":      token,
		"sessionKey": params.SessionKey,
		"expiresIn":  1800, // seconds
	}))
}

type remoteAttachParams struct {
	Token string `json:"token"`
}

func (m *RemoteMethods) handleAttach(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params remoteAttachParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Token == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "token")))
		return
	}

	remoteTokensMu.RLock()
	rt, ok := remoteTokens[params.Token]
	remoteTokensMu.RUnlock()

	if !ok || time.Now().After(rt.ExpiresAt) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "token")))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"sessionKey": rt.SessionKey,
		"userId":     rt.UserID,
		"attached":   true,
	}))
}
