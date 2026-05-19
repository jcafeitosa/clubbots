package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
)

// RemoteControlSession represents an active remote-controlled agent session.
type RemoteControlSession struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	SessionKey string    `json:"session_key"`
	UserID     string    `json:"user_id"`
	Surface    string    `json:"surface"` // "web", "mobile", "desktop", "cli"
	Connected  bool      `json:"connected"`
	LastActive time.Time `json:"last_active"`
	Status     string    `json:"status"` // "idle", "running", "waiting_input", "done"
}

// RemoteControlManager enables cross-surface session control.
// Inspired by Claude Code's Remote Control feature.
type RemoteControlManager struct {
	mu       sync.RWMutex
	sessions map[string]*RemoteControlSession // id → session
	eventBus bus.EventPublisher
}

// NewRemoteControlManager creates a remote control manager.
func NewRemoteControlManager(eventBus bus.EventPublisher) *RemoteControlManager {
	return &RemoteControlManager{
		sessions: make(map[string]*RemoteControlSession),
		eventBus: eventBus,
	}
}

// Register adds a session for remote control.
func (m *RemoteControlManager) Register(session *RemoteControlSession) {
	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	m.broadcast(RemoteControlEvent{
		Type:    "session.registered",
		Session: session,
	})
}

// Connect marks a session as connected from a remote surface.
func (m *RemoteControlManager) Connect(id, surface string) (*RemoteControlSession, error) {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("remote_control: session %s not found", id)
	}
	session.Connected = true
	session.Surface = surface
	session.LastActive = time.Now()
	m.mu.Unlock()

	m.broadcast(RemoteControlEvent{
		Type:    "session.connected",
		Session: session,
	})
	return session, nil
}

// Disconnect marks a session as disconnected.
func (m *RemoteControlManager) Disconnect(id string) {
	m.mu.Lock()
	if session, ok := m.sessions[id]; ok {
		session.Connected = false
	}
	m.mu.Unlock()
}

// SendMessage injects a user message into a remote session.
func (m *RemoteControlManager) SendMessage(ctx context.Context, id, message string) error {
	m.mu.RLock()
	session, ok := m.sessions[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("remote_control: session %s not found", id)
	}
	if !session.Connected {
		return fmt.Errorf("remote_control: session %s not connected", id)
	}

	session.LastActive = time.Now()

	m.broadcast(RemoteControlEvent{
		Type:    "message.received",
		Session: session,
		Message: message,
	})
	return nil
}

// UpdateStatus updates session status.
func (m *RemoteControlManager) UpdateStatus(id, status string) {
	m.mu.Lock()
	if session, ok := m.sessions[id]; ok {
		session.Status = status
		session.LastActive = time.Now()
	}
	m.mu.Unlock()
}

// List returns all remote-controllable sessions for a user.
func (m *RemoteControlManager) List(userID string) []*RemoteControlSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*RemoteControlSession
	for _, s := range m.sessions {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	return result
}

// Get returns a session by ID.
func (m *RemoteControlManager) Get(id string) *RemoteControlSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Cleanup removes stale disconnected sessions.
func (m *RemoteControlManager) Cleanup(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	for id, s := range m.sessions {
		if !s.Connected && time.Since(s.LastActive) > maxAge {
			delete(m.sessions, id)
			removed++
		}
	}
	return removed
}

func (m *RemoteControlManager) broadcast(event RemoteControlEvent) {
	if m.eventBus == nil {
		return
	}
	data, _ := json.Marshal(event)
	m.eventBus.Broadcast(bus.Event{
		Name:    "remote_control",
		Payload: string(data),
	})
}

// RemoteControlEvent is emitted on the event bus for remote control actions.
type RemoteControlEvent struct {
	Type    string                 `json:"type"`
	Session *RemoteControlSession  `json:"session,omitempty"`
	Message string                 `json:"message,omitempty"`
}
