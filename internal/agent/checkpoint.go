package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Checkpoint captures agent run state for pause/resume.
// Inspired by Claude Managed Agents checkpointing feature.
type Checkpoint struct {
	RunID       string    `json:"run_id"`
	SessionKey  string    `json:"session_key"`
	AgentID     string    `json:"agent_id"`
	Iteration   int       `json:"iteration"`
	Message     string    `json:"message"`     // last user message
	Response    string    `json:"response"`    // accumulated response so far
	State       string    `json:"state"`       // serialized pipeline state (JSON)
	CreatedAt   time.Time `json:"created_at"`
	ToolResults []string  `json:"tool_results"` // completed tool outputs
}

// CheckpointStore persists and retrieves agent checkpoints.
type CheckpointStore interface {
	Save(ctx context.Context, cp *Checkpoint) error
	Load(ctx context.Context, sessionKey string) (*Checkpoint, error)
	Delete(ctx context.Context, sessionKey string) error
	List(ctx context.Context, agentID string) ([]*Checkpoint, error)
}

// SessionCheckpointStore implements CheckpointStore using session metadata.
type SessionCheckpointStore struct {
	sessions store.SessionStore
}

func NewSessionCheckpointStore(s store.SessionStore) *SessionCheckpointStore {
	return &SessionCheckpointStore{sessions: s}
}

func (s *SessionCheckpointStore) Save(ctx context.Context, cp *Checkpoint) error {
	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("checkpoint marshal: %w", err)
	}
	s.sessions.SetSessionMetadata(ctx, cp.SessionKey, map[string]string{
		"checkpoint":    string(data),
		"checkpoint_at": cp.CreatedAt.Format(time.RFC3339),
	})
	return nil
}

func (s *SessionCheckpointStore) Load(ctx context.Context, sessionKey string) (*Checkpoint, error) {
	meta := s.sessions.GetSessionMetadata(ctx, sessionKey)
	if meta == nil {
		return nil, nil
	}
	data, ok := meta["checkpoint"]
	if !ok || data == "" {
		return nil, nil
	}
	var cp Checkpoint
	if err := json.Unmarshal([]byte(data), &cp); err != nil {
		return nil, fmt.Errorf("checkpoint unmarshal: %w", err)
	}
	return &cp, nil
}

func (s *SessionCheckpointStore) Delete(ctx context.Context, sessionKey string) error {
	s.sessions.SetSessionMetadata(ctx, sessionKey, map[string]string{
		"checkpoint":    "",
		"checkpoint_at": "",
	})
	return nil
}

func (s *SessionCheckpointStore) List(ctx context.Context, agentID string) ([]*Checkpoint, error) {
	// List sessions with checkpoints for this agent
	result := s.sessions.ListPagedRich(ctx, store.SessionListOpts{
		AgentID: agentID,
		Limit:   100,
	})
	var cps []*Checkpoint
	for _, sess := range result.Sessions {
		if sess.Metadata != nil {
			if data, ok := sess.Metadata["checkpoint"]; ok && data != "" {
				var cp Checkpoint
				if json.Unmarshal([]byte(data), &cp) == nil {
					cps = append(cps, &cp)
				}
			}
		}
	}
	return cps, nil
}

// SaveCheckpoint creates a checkpoint from current run state.
func SaveCheckpoint(ctx context.Context, store CheckpointStore, runID, sessionKey, agentID string, iteration int, message, response, state string) error {
	cp := &Checkpoint{
		RunID:      runID,
		SessionKey: sessionKey,
		AgentID:    agentID,
		Iteration:  iteration,
		Message:    message,
		Response:   response,
		State:      state,
		CreatedAt:  time.Now(),
	}
	return store.Save(ctx, cp)
}
