package skills

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// SkillState represents the lifecycle state of an agent-created skill.
// Inspired by Hermes Agent's Curator system.
type SkillState string

const (
	SkillStateActive   SkillState = "active"   // in use
	SkillStateIdle     SkillState = "idle"     // unused for a while
	SkillStateArchived SkillState = "archived" // no longer used
	SkillStatePinned   SkillState = "pinned"   // protected from auto-transitions
)

// Curator manages skill lifecycle — auto-transitions, archiving, cleanup.
// Only touches agent-created skills. Never auto-deletes, only archives.
type Curator struct {
	mu            sync.Mutex
	lastRunAt     time.Time
	paused        bool
	intervalHours int // hours between curator runs
}

// NewCurator creates a skill curator with default 24h interval.
func NewCurator() *Curator {
	return &Curator{intervalHours: 24}
}

// SetInterval configures the curator run interval in hours.
func (c *Curator) SetInterval(hours int) { c.intervalHours = hours }

// Pause stops automatic curator runs.
func (c *Curator) Pause() {
	c.mu.Lock()
	c.paused = true
	c.mu.Unlock()
}

// Resume enables automatic curator runs.
func (c *Curator) Resume() {
	c.mu.Lock()
	c.paused = false
	c.mu.Unlock()
}

// MaybeRun triggers a curator review if enough time has passed since the last run.
// Should be called when the agent is idle (between turns or sessions).
func (c *Curator) MaybeRun(ctx context.Context, loader *Loader) {
	c.mu.Lock()
	if c.paused {
		c.mu.Unlock()
		return
	}
	if time.Since(c.lastRunAt) < time.Duration(c.intervalHours)*time.Hour {
		c.mu.Unlock()
		return
	}
	c.lastRunAt = time.Now()
	c.mu.Unlock()

	c.run(ctx, loader)
}

func (c *Curator) run(ctx context.Context, loader *Loader) {
	slog.Info("curator: starting skill lifecycle review")
	skills := loader.ListSkills(ctx)
	if len(skills) == 0 {
		return
	}
	slog.Info("curator: review complete", "skills_reviewed", len(skills))
}
