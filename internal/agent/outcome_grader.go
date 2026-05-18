package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// OutcomeRubric defines success criteria for goal evaluation.
// Inspired by Claude Managed Agents outcomes feature.
type OutcomeRubric struct {
	Goal        string          `json:"goal"`        // what to achieve
	Criteria    []OutcomeCriterion `json:"criteria"` // measurable checks
	MinScore    float64         `json:"min_score"`   // 0.0-1.0 threshold (default 0.8)
	MaxAttempts int             `json:"max_attempts"` // max self-correction loops (default 3)
}

// OutcomeCriterion is a single measurable check within a rubric.
type OutcomeCriterion struct {
	Name        string  `json:"name"`        // e.g. "build_passes"
	Description string  `json:"description"` // e.g. "go build ./... succeeds"
	Weight      float64 `json:"weight"`      // 0.0-1.0, weights should sum to 1.0
}

// OutcomeResult is the grader's evaluation of agent output.
type OutcomeResult struct {
	Passed      bool              `json:"passed"`
	Score       float64           `json:"score"`
	CriterionResults []OutcomeCriterionResult `json:"criteria"`
	Feedback    string            `json:"feedback"` // what to improve
	Attempts    int               `json:"attempts"`
}

// OutcomeCriterionResult is one criterion's evaluation.
type OutcomeCriterionResult struct {
	Name   string  `json:"name"`
	Passed bool    `json:"passed"`
	Score  float64 `json:"score"` // 0.0-1.0
	Note   string  `json:"note"`  // specific feedback
}

// OutcomeGrader evaluates agent output against a rubric using a separate LLM call.
// Inspired by Claude's "separate grader in its own context window" pattern.
type OutcomeGrader struct {
	provider providers.Provider
	model    string
}

// NewOutcomeGrader creates a grader with the given provider/model.
func NewOutcomeGrader(provider providers.Provider, model string) *OutcomeGrader {
	return &OutcomeGrader{provider: provider, model: model}
}

// Evaluate runs the grader against the agent's output.
func (g *OutcomeGrader) Evaluate(ctx context.Context, rubric OutcomeRubric, agentOutput string) (*OutcomeResult, error) {
	if g.provider == nil {
		return nil, fmt.Errorf("outcome grader: no provider configured")
	}
	if rubric.MaxAttempts <= 0 {
		rubric.MaxAttempts = 3
	}
	if rubric.MinScore <= 0 {
		rubric.MinScore = 0.8
	}

	prompt := buildGraderPrompt(rubric, agentOutput)

	resp, err := g.provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "system", Content: graderSystemPrompt},
			{Role: "user", Content: prompt},
		},
		Model:   g.model,
		Options: map[string]any{"max_tokens": 1024},
	})
	if err != nil {
		return nil, fmt.Errorf("outcome grader: LLM call failed: %w", err)
	}

	var result OutcomeResult
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		// Try to extract JSON from response
		cleaned := extractJSON(resp.Content)
		if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
			return &OutcomeResult{
				Passed:   false,
				Score:    0,
				Feedback: fmt.Sprintf("grader parse error: %v. Raw: %s", err, resp.Content[:200]),
			}, nil
		}
	}

	result.Attempts = 1
	return &result, nil
}

// EvaluateWithRetry runs the grader and requests agent retry if score < minScore.
func (g *OutcomeGrader) EvaluateWithRetry(ctx context.Context, rubric OutcomeRubric, runAgent func() (string, error)) (*OutcomeResult, string, error) {
	var lastOutput string
	var lastResult *OutcomeResult

	for attempt := 1; attempt <= rubric.MaxAttempts; attempt++ {
		output, err := runAgent()
		if err != nil {
			return nil, "", fmt.Errorf("attempt %d: %w", attempt, err)
		}
		lastOutput = output

		result, err := g.Evaluate(ctx, rubric, output)
		if err != nil {
			return nil, "", err
		}
		result.Attempts = attempt
		lastResult = result

		if result.Passed && result.Score >= rubric.MinScore {
			return result, output, nil
		}

		// Feed feedback back to agent context for next attempt
		if attempt < rubric.MaxAttempts {
			ctx = context.WithValue(ctx, contextKeyGraderFeedback, result.Feedback)
		}
	}

	return lastResult, lastOutput, nil
}

type graderFeedbackKey struct{}

var contextKeyGraderFeedback = graderFeedbackKey{}

const graderSystemPrompt = `You are a strict outcome evaluator. Grade the agent's output against the rubric.
Respond with ONLY valid JSON in this exact format:
{
  "passed": true/false,
  "score": 0.0-1.0,
  "criteria": [
    {"name": "criterion_name", "passed": true/false, "score": 0.0-1.0, "note": "why"}
  ],
  "feedback": "What needs improvement if score < minScore"
}
Be objective. Score each criterion independently. Do not be lenient.`

func buildGraderPrompt(rubric OutcomeRubric, output string) string {
	criteriaJSON, _ := json.MarshalIndent(rubric.Criteria, "", "  ")
	return fmt.Sprintf(`Grade this agent output against the rubric.

GOAL: %s
MINIMUM SCORE: %.0f%%

CRITERIA:
%s

AGENT OUTPUT:
%s

Evaluate each criterion. Return JSON only.`, rubric.Goal, rubric.MinScore*100, string(criteriaJSON), output)
}

func extractJSON(s string) string {
	// Find first { and last }
	start := -1
	end := -1
	for i, c := range s {
		if c == '{' && start == -1 {
			start = i
		}
		if c == '}' {
			end = i
		}
	}
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
