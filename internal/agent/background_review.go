package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// BackgroundReview evaluates agent turns and suggests memory/skill updates.
// Inspired by Hermes Agent's background_review.py — forks a lightweight
// agent after each turn to evaluate what should be saved.
type BackgroundReview struct {
	provider providers.Provider
	model    string
	enabled  bool
}

// NewBackgroundReview creates a background review evaluator.
func NewBackgroundReview(provider providers.Provider, model string) *BackgroundReview {
	return &BackgroundReview{provider: provider, model: model, enabled: true}
}

// Enable toggles background review on/off.
func (r *BackgroundReview) Enable(v bool) { r.enabled = v }

// ReviewResult contains the background review evaluation.
type ReviewResult struct {
	ShouldSaveMemory bool   `json:"should_save_memory"`
	MemoryContent    string `json:"memory_content,omitempty"`
	MemoryTopic      string `json:"memory_topic,omitempty"`
	ShouldSaveSkill  bool   `json:"should_save_skill"`
	SkillName        string `json:"skill_name,omitempty"`
	SkillContent     string `json:"skill_content,omitempty"`
	Summary          string `json:"summary"` // 1-2 sentence turn summary
}

// Review evaluates a turn and decides whether to save memory or create a skill.
// Runs asynchronously — never blocks the main conversation loop.
func (r *BackgroundReview) Review(ctx context.Context, turn TurnSnapshot) {
	if !r.enabled || r.provider == nil {
		return
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := r.evaluate(bgCtx, turn)
		if err != nil {
			slog.Debug("background_review: evaluation failed", "error", err)
			return
		}

		if result.ShouldSaveMemory && result.MemoryContent != "" {
			slog.Info("background_review: memory suggestion",
				"topic", result.MemoryTopic,
				"preview", truncateStr(result.MemoryContent, 80),
			)
		}
		if result.ShouldSaveSkill && result.SkillName != "" {
			slog.Info("background_review: skill suggestion",
				"name", result.SkillName,
				"preview", truncateStr(result.SkillContent, 80),
			)
		}
	}()
}

func (r *BackgroundReview) evaluate(ctx context.Context, turn TurnSnapshot) (*ReviewResult, error) {
	prompt := buildBackgroundReviewPrompt(turn)
	resp, err := r.provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "system", Content: bgReviewSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Model:   r.model,
		Options: map[string]any{"max_tokens": 512},
	})
	if err != nil {
		return nil, err
	}

	return parseReviewResponse(resp.Content), nil
}

// TurnSnapshot captures a single conversation turn for review.
type TurnSnapshot struct {
	UserMessage    string `json:"user_message"`
	AgentResponse  string `json:"agent_response"`
	ToolsUsed      []string `json:"tools_used"`
	FilesChanged   []string `json:"files_changed"`
	SessionKey     string `json:"session_key"`
	AgentID        string `json:"agent_id"`
}

const bgReviewSystemPrompt = `You are a background reviewer for an AI agent. After each turn, evaluate if anything should be saved as:
1. Memory — a fact, preference, or pattern worth remembering across sessions
2. Skill — a repeatable workflow or pattern the agent used

Respond with ONLY valid JSON:
{
  "should_save_memory": bool,
  "memory_content": "string (if true)",
  "memory_topic": "string (if true)",
  "should_save_skill": bool,
  "skill_name": "string (if true)",
  "skill_content": "string (if true)",
  "summary": "1-2 sentence turn summary"
}

Be conservative — only save genuinely new or noteworthy things.
Don't save trivial facts or one-off patterns.`

func buildBackgroundReviewPrompt(turn TurnSnapshot) string {
	return fmt.Sprintf(`Evaluate this agent turn:

USER: %s

AGENT RESPONSE: %s

TOOLS USED: %s
FILES CHANGED: %s

Should any memory or skill be saved?`,
		truncateStr(turn.UserMessage, 500),
		truncateStr(turn.AgentResponse, 1000),
		strings.Join(turn.ToolsUsed, ", "),
		strings.Join(turn.FilesChanged, ", "),
	)
}

func parseReviewResponse(content string) *ReviewResult {
	// Try to find JSON in the response
	result := &ReviewResult{}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		// Simple manual parse (avoid json.Unmarshal dependency here)
		result.Summary = extractJSONField(jsonStr, "summary")
		result.MemoryContent = extractJSONField(jsonStr, "memory_content")
		result.MemoryTopic = extractJSONField(jsonStr, "memory_topic")
		result.SkillName = extractJSONField(jsonStr, "skill_name")
		result.SkillContent = extractJSONField(jsonStr, "skill_content")
		result.ShouldSaveMemory = strings.Contains(jsonStr, `"should_save_memory": true`)
		result.ShouldSaveSkill = strings.Contains(jsonStr, `"should_save_skill": true`)
	}
	return result
}

func extractJSONField(json, key string) string {
	search := fmt.Sprintf(`"%s": "`, key)
	idx := strings.Index(json, search)
	if idx < 0 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(json[start:], `"`)
	if end < 0 {
		return json[start:]
	}
	return json[start : start+end]
}
