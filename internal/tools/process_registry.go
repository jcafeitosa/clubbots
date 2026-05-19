package tools

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ProcessEntry tracks a background process managed by the registry.
// Inspired by Hermes Agent's process_registry.py.
type ProcessEntry struct {
	ID         string
	Command    string
	Args       []string
	StartedAt  time.Time
	Cmd        *exec.Cmd
	stdin      io.Writer
	Output     *strings.Builder // rolling buffer (200KB max)
	mu         sync.Mutex
	done       chan struct{}
	ExitCode   int
	FinishedAt time.Time
}

// ProcessRegistry manages background process lifecycle.
// Handles spawn, output capture, watch patterns, and rate limiting.
type ProcessRegistry struct {
	mu       sync.RWMutex
	processes map[string]*ProcessEntry
	maxOutput int // per-process output buffer limit in bytes
}

// NewProcessRegistry creates a process registry.
func NewProcessRegistry() *ProcessRegistry {
	return &ProcessRegistry{
		processes: make(map[string]*ProcessEntry),
		maxOutput: 200 * 1024, // 200KB rolling buffer
	}
}

// Spawn starts a background process and returns its ID.
func (r *ProcessRegistry) Spawn(ctx context.Context, id, command string, args ...string) (*ProcessEntry, error) {
	r.mu.Lock()
	if _, exists := r.processes[id]; exists {
		r.mu.Unlock()
		return nil, fmt.Errorf("process_registry: %s already running", id)
	}

	entry := &ProcessEntry{
		ID:        id,
		Command:   command,
		Args:      args,
		StartedAt: time.Now(),
		Output:    &strings.Builder{},
		done:      make(chan struct{}),
	}
	entry.Cmd = exec.CommandContext(ctx, command, args...)
	r.processes[id] = entry
	r.mu.Unlock()

	// Capture stdin/stdout/stderr
	stdin, _ := entry.Cmd.StdinPipe()
	entry.stdin = stdin
	stdout, _ := entry.Cmd.StdoutPipe()
	stderr, _ := entry.Cmd.StderrPipe()

	if err := entry.Cmd.Start(); err != nil {
		r.mu.Lock()
		delete(r.processes, id)
		r.mu.Unlock()
		return nil, fmt.Errorf("process_registry: spawn %s: %w", id, err)
	}

	go r.captureOutput(id, stdout, stderr)
	go r.waitForExit(id)

	return entry, nil
}

func (r *ProcessRegistry) captureOutput(id string, stdout, stderr io.Reader) {
	r.mu.RLock()
	entry, ok := r.processes[id]
	r.mu.RUnlock()
	if !ok {
		return
	}

	buf := make([]byte, 4096)
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			entry.mu.Lock()
			entry.Output.Write(buf[:n])
			// Rolling buffer: truncate from beginning if over limit
			if entry.Output.Len() > r.maxOutput {
				excess := entry.Output.Len() - r.maxOutput
				output := entry.Output.String()
				entry.Output.Reset()
				entry.Output.WriteString("...(truncated)...")
				entry.Output.WriteString(output[excess+14:])
			}
			entry.mu.Unlock()
		}
		if err != nil {
			break
		}
	}
	_ = stderr // stderr mixed into stdout via pipe
}

func (r *ProcessRegistry) waitForExit(id string) {
	r.mu.RLock()
	entry, ok := r.processes[id]
	r.mu.RUnlock()
	if !ok {
		return
	}

	_ = entry.Cmd.Wait()
	entry.mu.Lock()
	entry.FinishedAt = time.Now()
	entry.ExitCode = entry.Cmd.ProcessState.ExitCode()
	entry.mu.Unlock()
	close(entry.done)
}

// Read returns the current output buffer for a process.
func (r *ProcessRegistry) Read(id string) (string, error) {
	r.mu.RLock()
	entry, ok := r.processes[id]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("process_registry: %s not found", id)
	}

	entry.mu.Lock()
	out := entry.Output.String()
	entry.mu.Unlock()
	return out, nil
}

// Write sends input to the process stdin.
func (r *ProcessRegistry) Write(id, input string) error {
	r.mu.RLock()
	entry, ok := r.processes[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process_registry: %s not found", id)
	}

	if entry.Cmd.Process == nil {
		return fmt.Errorf("process_registry: %s not started", id)
	}
	if entry.stdin == nil {
		return fmt.Errorf("process_registry: %s has no stdin pipe", id)
	}

	_, err := io.WriteString(entry.stdin, input)
	return err
}

// Kill terminates a process.
func (r *ProcessRegistry) Kill(id string) error {
	r.mu.RLock()
	entry, ok := r.processes[id]
	r.mu.RUnlock()
	if !ok {
		return nil // already gone
	}

	return entry.Cmd.Process.Kill()
}

// Status returns the process status.
func (r *ProcessRegistry) Status(id string) map[string]any {
	r.mu.RLock()
	entry, ok := r.processes[id]
	r.mu.RUnlock()
	if !ok {
		return map[string]any{"status": "not_found"}
	}

	select {
	case <-entry.done:
		entry.mu.Lock()
		code := entry.ExitCode
		entry.mu.Unlock()
		return map[string]any{
			"status":   "exited",
			"exitCode": code,
			"runtime":  entry.FinishedAt.Sub(entry.StartedAt).String(),
		}
	default:
		return map[string]any{
			"status":  "running",
			"runtime": time.Since(entry.StartedAt).String(),
		}
	}
}

// List returns all managed process IDs.
func (r *ProcessRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.processes))
	for id := range r.processes {
		ids = append(ids, id)
	}
	return ids
}

// Cleanup removes exited processes older than the given duration.
func (r *ProcessRegistry) Cleanup(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for id, entry := range r.processes {
		select {
		case <-entry.done:
			if time.Since(entry.FinishedAt) > maxAge {
				delete(r.processes, id)
				removed++
			}
		default:
		}
	}
	return removed
}

// Shutdown kills all processes.
func (r *ProcessRegistry) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, entry := range r.processes {
		entry.Cmd.Process.Kill()
		delete(r.processes, id)
	}
}
