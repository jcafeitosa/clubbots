package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// TrajectoryCompressor post-processes agent runs for training data / context compression.
// Inspired by Hermes Agent's trajectory_compressor.py — head/tail protection pattern.
type TrajectoryCompressor struct {
	headTurns    int // protect first N turns (system + human + first assistant)
	tailTurns    int // protect last N turns
	targetTokens int // target token count (0 = no limit)
	summaryTokens int // max tokens for the summary placeholder
}

// NewTrajectoryCompressor creates a compressor with defaults.
func NewTrajectoryCompressor() *TrajectoryCompressor {
	return &TrajectoryCompressor{
		headTurns:     3,
		tailTurns:     4,
		targetTokens:  15250,
		summaryTokens: 750,
	}
}

// CompressResult is the output of trajectory compression.
type CompressResult struct {
	Messages        []providers.Message `json:"messages"`
	OriginalTokens  int                 `json:"original_tokens"`
	CompressedTokens int                `json:"compressed_tokens"`
	SavingsPercent  float64             `json:"savings_percent"`
	TurnsCompressed int                 `json:"turns_compressed"`
	SummaryText     string              `json:"summary_text,omitempty"`
}

// Compress reduces a conversation trajectory by summarizing middle turns.
// Head and tail turns are preserved; middle turns are replaced with a summary.
func (c *TrajectoryCompressor) Compress(ctx context.Context, messages []providers.Message, provider providers.Provider, model string) (*CompressResult, error) {
	if len(messages) <= c.headTurns+c.tailTurns {
		return &CompressResult{
			Messages:       messages,
			OriginalTokens: len(messages),
		}, nil
	}

	head := messages[:c.headTurns]
	tail := messages[len(messages)-c.tailTurns:]
	middle := messages[c.headTurns : len(messages)-c.tailTurns]

	// Build summary of middle turns
	middleText := buildMiddleText(middle)
	summary, err := c.summarize(ctx, provider, model, middleText)
	if err != nil {
		// Fallback: return original messages
		return &CompressResult{
			Messages:       messages,
			OriginalTokens: len(messages),
		}, nil
	}

	// Replace middle with summary placeholder
	compressed := make([]providers.Message, 0, len(head)+1+len(tail))
	compressed = append(compressed, head...)
	compressed = append(compressed, providers.Message{
		Role:    "user",
		Content: fmt.Sprintf("[CONTEXT SUMMARY — %d turns compressed]\n%s", len(middle)/2, summary),
	})
	compressed = append(compressed, tail...)

	savings := float64(len(middle)) / float64(len(messages)) * 100
	return &CompressResult{
		Messages:         compressed,
		OriginalTokens:   len(messages),
		CompressedTokens: len(compressed),
		SavingsPercent:   savings,
		TurnsCompressed:  len(middle) / 2,
		SummaryText:      summary,
	}, nil
}

func buildMiddleText(messages []providers.Message) string {
	var sb strings.Builder
	for i, msg := range messages {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s", msg.Role, truncateStr(msg.Content, 500)))
	}
	return sb.String()
}

func (c *TrajectoryCompressor) summarize(ctx context.Context, provider providers.Provider, model, text string) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("no provider for summarization")
	}
	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "system", Content: "Summarize this conversation segment in 2-3 sentences. Focus on: key decisions, files changed, problems solved, and remaining tasks."},
			{Role: "user", Content: text},
		},
		Model:   model,
		Options: map[string]any{"max_tokens": c.summaryTokens},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// truncateStr is defined in loop_tracing.go — reused here.
