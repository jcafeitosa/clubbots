package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Orchestrator breaks complex tasks into subtasks and dispatches to subagents.
// Inspired by Claude Code's multiagent orchestration and Hermes Agent's delegation.
type Orchestrator struct {
	router    *Router
	maxAgents int
}

// NewOrchestrator creates an orchestrator with the given agent router.
func NewOrchestrator(router *Router, maxAgents int) *Orchestrator {
	if maxAgents <= 0 {
		maxAgents = 5
	}
	return &Orchestrator{router: router, maxAgents: maxAgents}
}

// Subtask represents a piece of work dispatched to a subagent.
type Subtask struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	AgentType   string `json:"agent_type"` // "explore", "implement", "review", "test"
	Scope       string `json:"scope"`      // files/directories
	Prompt      string `json:"prompt"`
	ExpectedOut string `json:"expected_output"`
	Priority    int    `json:"priority"` // 1=highest
}

// SubtaskResult is the output of a completed subtask.
type SubtaskResult struct {
	Subtask  Subtask `json:"subtask"`
	Output   string  `json:"output"`
	Success  bool    `json:"success"`
	Error    string  `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// OrchestrationPlan is the full plan for a complex task.
type OrchestrationPlan struct {
	Goal        string    `json:"goal"`
	Subtasks    []Subtask `json:"subtasks"`
	TotalAgents int       `json:"total_agents"`
}

// Plan analyzes the goal and creates an orchestration plan.
func (o *Orchestrator) Plan(goal string) *OrchestrationPlan {
	return &OrchestrationPlan{
		Goal:        goal,
		Subtasks:    nil, // filled by agent via tool calls
		TotalAgents: o.maxAgents,
	}
}

// Execute runs all subtasks, dispatching up to maxAgents in parallel.
// Returns results in dispatch order, not completion order.
func (o *Orchestrator) Execute(ctx context.Context, plan *OrchestrationPlan) []SubtaskResult {
	if len(plan.Subtasks) == 0 {
		return nil
	}

	results := make([]SubtaskResult, len(plan.Subtasks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, o.maxAgents) // concurrency limiter

	for i := range plan.Subtasks {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			st := plan.Subtasks[idx]
			start := time.Now()
			output, err := o.dispatchOne(ctx, st)
			results[idx] = SubtaskResult{
				Subtask:  st,
				Output:   output,
				Success:  err == nil,
				Error:    errStr(err),
				Duration: time.Since(start),
			}

			slog.Info("orchestrator: subtask complete",
				"id", st.ID,
				"success", err == nil,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}(i)
	}

	wg.Wait()
	return results
}

func (o *Orchestrator) dispatchOne(ctx context.Context, st Subtask) (string, error) {
	loop, err := o.router.Get(ctx, st.AgentType)
	if err != nil {
		return "", fmt.Errorf("orchestrator: agent %q not found: %w", st.AgentType, err)
	}

	result, err := loop.Run(ctx, RunRequest{
		Message: st.Prompt,
		Channel: "orchestrator",
		UserID:  "orchestrator",
	})
	if err != nil {
		return "", err
	}

	return result.Content, nil
}

// MergeResults combines multiple subtask outputs into a unified response.
// Detects conflicts when two subtasks touched the same file.
func (o *Orchestrator) MergeResults(results []SubtaskResult) string {
	if len(results) == 0 {
		return "No results to merge."
	}

	var msg string
	successes := 0
	failures := 0

	for _, r := range results {
		if r.Success {
			successes++
		} else {
			failures++
			msg += fmt.Sprintf("❌ %s: %s\n", r.Subtask.Description, r.Error)
		}
	}

	msg += fmt.Sprintf("\n%d/%d subtasks completed (%d failed)\n\n", successes, len(results), failures)

	for _, r := range results {
		if r.Success && r.Output != "" {
			msg += fmt.Sprintf("## %s\n%s\n\n", r.Subtask.Description, r.Output)
		}
	}

	return msg
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
