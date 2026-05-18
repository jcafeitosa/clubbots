package methods

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// FeedbackMethods handles feedback.send.
type FeedbackMethods struct{}

func NewFeedbackMethods() *FeedbackMethods {
	return &FeedbackMethods{}
}

func (m *FeedbackMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodFeedbackSend, m.handleSend)
}

type feedbackSendParams struct {
	Message    string `json:"message"`
	SessionKey string `json:"sessionKey,omitempty"`
	AgentID    string `json:"agentId,omitempty"`
	Category   string `json:"category,omitempty"` // "bug", "feature", "improvement", "other"
}

func (m *FeedbackMethods) handleSend(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params feedbackSendParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Message == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "message")))
		return
	}

	slog.Info("feedback.submitted",
		"message", params.Message,
		"sessionKey", params.SessionKey,
		"agentId", params.AgentID,
		"category", params.Category,
		"userId", client.UserID(),
		"tenantId", client.TenantID().String(),
	)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"status": "received",
	}))
}
