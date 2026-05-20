package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// OrchestrateTool dispatches multiple subtasks in parallel via subagent spawning.
// Available to agents in ModeTeam and ModeDelegate.
type OrchestrateTool struct {
	subagentMgr *SubagentManager
	parentID    string
	depth       int
}

func NewOrchestrateTool(manager *SubagentManager, parentID string, depth int) *OrchestrateTool {
	return &OrchestrateTool{
		subagentMgr: manager,
		parentID:    parentID,
		depth:       depth,
	}
}

func (t *OrchestrateTool) Name() string { return "orchestrate" }

func (t *OrchestrateTool) Description() string {
	return "Break a complex task into subtasks and dispatch them in parallel to subagents. Use this for multi-step work that can be parallelized — research + implementation, multiple file edits, or independent investigations."
}

func (t *OrchestrateTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"goal": map[string]any{
				"type":        "string",
				"description": "The overall goal this orchestration should achieve",
			},
			"subtasks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"description": map[string]any{
							"type":        "string",
							"description": "One-line description of what this subtask should do",
						},
						"prompt": map[string]any{
							"type":        "string",
							"description": "Full task prompt for the subagent — include specific files, expected output format, and success criteria",
						},
						"label": map[string]any{
							"type":        "string",
							"description": "Short label for tracking (e.g. 'research-auth', 'implement-login')",
						},
						"model": map[string]any{
							"type":        "string",
							"description": "Optional model override per subtask (e.g. haiku for simple, sonnet for complex)",
						},
					},
					"required": []string{"prompt", "label"},
				},
				"description": "List of subtasks to dispatch in parallel (max 5)",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"async", "sync"},
				"description": "'async' returns immediately (results announced later). 'sync' waits for all subtasks (default).",
			},
		},
		"required": []string{"goal", "subtasks"},
	}
}

func (t *OrchestrateTool) Execute(ctx context.Context, args map[string]any) *Result {
	goal, _ := args["goal"].(string)
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "sync"
	}

	rawSubtasks, ok := args["subtasks"].([]any)
	if !ok || len(rawSubtasks) == 0 {
		return ErrorResult("subtasks must be a non-empty array")
	}
	if len(rawSubtasks) > 5 {
		return ErrorResult("max 5 subtasks per orchestration — break into smaller batches")
	}

	subtasks := make([]orchestrateSubtask, 0, len(rawSubtasks))
	for _, raw := range rawSubtasks {
		m, ok := raw.(map[string]any)
		if !ok {
			return ErrorResult("each subtask must be an object with 'prompt' and 'label'")
		}
		prompt, _ := m["prompt"].(string)
		label, _ := m["label"].(string)
		desc, _ := m["description"].(string)
		model, _ := m["model"].(string)
		if prompt == "" || label == "" {
			return ErrorResult("each subtask requires 'prompt' and 'label'")
		}
		subtasks = append(subtasks, orchestrateSubtask{
			Description: desc,
			Prompt:      prompt,
			Label:       label,
			Model:       model,
		})
	}

	if mode == "async" {
		return t.executeAsync(ctx, goal, subtasks)
	}
	return t.executeSync(ctx, goal, subtasks)
}

type orchestrateSubtask struct {
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Label       string `json:"label"`
	Model       string `json:"model"`
}

func (t *OrchestrateTool) executeAsync(ctx context.Context, goal string, subtasks []orchestrateSubtask) *Result {
	channel := ToolChannelFromCtx(ctx)
	chatID := ToolChatIDFromCtx(ctx)
	peerKind := ToolPeerKindFromCtx(ctx)

	dispatched := make([]string, 0, len(subtasks))
	for _, st := range subtasks {
		msg, err := t.subagentMgr.Spawn(ctx, t.parentID, t.depth,
			st.Prompt, st.Label, st.Model,
			channel, chatID, peerKind, nil)
		if err != nil {
			return ErrorResult(fmt.Sprintf("failed to spawn '%s': %v", st.Label, err))
		}
		dispatched = append(dispatched, msg)
	}

	result, _ := json.Marshal(map[string]any{
		"goal":       goal,
		"dispatched": len(dispatched),
		"subtasks":   dispatched,
	})
	return NewResult(string(result))
}

func (t *OrchestrateTool) executeSync(ctx context.Context, goal string, subtasks []orchestrateSubtask) *Result {
	channel := ToolChannelFromCtx(ctx)
	chatID := ToolChatIDFromCtx(ctx)

	type subResult struct {
		Label      string `json:"label"`
		Output     string `json:"output"`
		Success    bool   `json:"success"`
		Error      string `json:"error,omitempty"`
		DurationMs int64  `json:"duration_ms"`
	}

	results := make([]subResult, len(subtasks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // max 5 concurrent

	for i := range subtasks {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			st := subtasks[idx]
			start := time.Now()
			output, iterations, err := t.subagentMgr.RunSync(ctx, t.parentID, t.depth,
				st.Prompt, st.Label, channel, chatID)
			results[idx] = subResult{
				Label:      st.Label,
				Output:     output,
				Success:    err == nil,
				Error:      errStr(err),
				DurationMs: time.Since(start).Milliseconds(),
			}
			_ = iterations
		}(i)
	}

	wg.Wait()

	var out strings.Builder
	out.WriteString(fmt.Sprintf("## Orchestration: %s\n\n", goal))
	successes, failures := 0, 0
	for _, r := range results {
		if r.Success {
			successes++
		} else {
			failures++
		}
	}
	out.WriteString(fmt.Sprintf("%d/%d subtasks completed", successes, len(results)))
	if failures > 0 {
		out.WriteString(fmt.Sprintf(" (%d failed)", failures))
	}
	out.WriteString("\n\n")

	for _, r := range results {
		status := "✅"
		if !r.Success {
			status = "❌"
		}
		out.WriteString(fmt.Sprintf("### %s %s (%dms)\n", status, r.Label, r.DurationMs))
		if r.Error != "" {
			out.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
		} else if r.Output != "" {
			out.WriteString(r.Output)
		}
		out.WriteString("\n\n")
	}

	return NewResult(out.String())
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
