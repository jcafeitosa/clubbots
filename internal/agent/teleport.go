package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TeleportSession enables cloud→local session migration.
// Inspired by Claude Code's --teleport feature.
type TeleportSession struct {
	Token      string    `json:"token"`
	SessionKey string    `json:"session_key"`
	AgentID    string    `json:"agent_id"`
	UserID     string    `json:"user_id"`
	Source     string    `json:"source"` // "web", "mobile", "desktop"
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// TeleportManager handles session migration between surfaces.
// Sessions can be teleported from web→CLI, CLI→desktop, etc.
type TeleportManager struct {
	mu       sync.RWMutex
	sessions map[string]*TeleportSession // token → session
}

// NewTeleportManager creates a teleport manager with background cleanup.
func NewTeleportManager() *TeleportManager {
	tm := &TeleportManager{
		sessions: make(map[string]*TeleportSession),
	}
	go tm.cleanupLoop()
	return tm
}

// Create generates a teleport token for a session.
func (tm *TeleportManager) Create(ctx context.Context, sessionKey, agentID, userID, source string) (*TeleportSession, error) {
	token, err := generateToken(16)
	if err != nil {
		return nil, err
	}

	ts := &TeleportSession{
		Token:      token,
		SessionKey: sessionKey,
		AgentID:    agentID,
		UserID:     userID,
		Source:     source,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}

	tm.mu.Lock()
	tm.sessions[token] = ts
	tm.mu.Unlock()

	return ts, nil
}

// Resolve looks up a teleport session by token.
// Returns nil if not found or expired.
func (tm *TeleportManager) Resolve(ctx context.Context, token string) *TeleportSession {
	tm.mu.RLock()
	ts, ok := tm.sessions[token]
	tm.mu.RUnlock()

	if !ok || time.Now().After(ts.ExpiresAt) {
		return nil
	}
	return ts
}

// Consume resolves and deletes the teleport session (one-time use).
func (tm *TeleportManager) Consume(ctx context.Context, token string) *TeleportSession {
	tm.mu.Lock()
	ts, ok := tm.sessions[token]
	if ok {
		delete(tm.sessions, token)
	}
	tm.mu.Unlock()

	if ts == nil || time.Now().After(ts.ExpiresAt) {
		return nil
	}
	return ts
}

// Revoke removes all teleport tokens for a session.
func (tm *TeleportManager) Revoke(sessionKey string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for token, ts := range tm.sessions {
		if ts.SessionKey == sessionKey {
			delete(tm.sessions, token)
		}
	}
}

// List returns all active teleport sessions for a user.
func (tm *TeleportManager) List(userID string) []*TeleportSession {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*TeleportSession
	for _, ts := range tm.sessions {
		if ts.UserID == userID && time.Now().Before(ts.ExpiresAt) {
			result = append(result, ts)
		}
	}
	return result
}

func (tm *TeleportManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		tm.mu.Lock()
		now := time.Now()
		for token, ts := range tm.sessions {
			if now.After(ts.ExpiresAt) {
				delete(tm.sessions, token)
			}
		}
		tm.mu.Unlock()
	}
}

func generateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("teleport token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
