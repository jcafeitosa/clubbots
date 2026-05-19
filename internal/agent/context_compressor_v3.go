package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// ContextCompressorV3 provides enhanced context compression with iterative updates.
// Inspired by Hermes Agent's context_compressor.py v3 improvements:
// - Structured summary template with Resolved/Pending question tracking
// - Token-budget tail protection instead of fixed message count
// - Tool output pruning before LLM summarization
// - Scaled summary budget proportional to compressed content
type ContextCompressorV3 struct {
	MaxTokens       int     // target max tokens after compression
	TailTokens      int     // tokens to protect at tail
	HeadTurns       int     // turns to protect at head
	SummaryFraction float64 // summary budget as fraction of compressed content
}

// NewContextCompressorV3 creates a v3 compressor with defaults.
func NewContextCompressorV3() *ContextCompressorV3 {
	return &ContextCompressorV3{
		MaxTokens:       15250,
		TailTokens:      2000,
		HeadTurns:       3,
		SummaryFraction: 0.15, // summary is ~15% of compressed content size
	}
}

// CompressResultV3 is the enhanced compression output.
type CompressResultV3 struct {
	Messages         []providers.Message `json:"messages"`
	Summary          string              `json:"summary"`
	TurnsCompressed  int                 `json:"turns_compressed"`
	OriginalTokens   int                 `json:"original_tokens"`
	CompressedTokens int                 `json:"compressed_tokens"`
	SavingsPercent   float64             `json:"savings_percent"`
	ResolvedItems    []string            `json:"resolved_items"`
	PendingItems     []string            `json:"pending_items"`
}

// Compress performs v3 compression with structured summary template.
func (c *ContextCompressorV3) Compress(ctx context.Context, messages []providers.Message, provider providers.Provider, model string) (*CompressResultV3, error) {
	if len(messages) <= c.HeadTurns+3 {
		return &CompressResultV3{Messages: messages}, nil
	}

	// Separate head, middle, tail
	head := messages[:c.HeadTurns]
	tail := c.extractTail(messages, c.TailTokens)
	middle := messages[c.HeadTurns : len(messages)-len(tail)]

	// Prune tool outputs from middle before summarization
	pruned := c.pruneToolOutputs(middle)

	// Build structured summary
	summaryBudget := int(float64(len(pruned)) * c.SummaryFraction)
	if summaryBudget < 200 {
		summaryBudget = 200
	}
	if summaryBudget > 2000 {
		summaryBudget = 2000
	}

	summary, resolved, pending, err := c.summarizeV3(ctx, provider, model, pruned, summaryBudget)
	if err != nil {
		return &CompressResultV3{Messages: messages}, nil
	}

	// Build compressed messages
	compressed := make([]providers.Message, 0, len(head)+1+len(tail))
	compressed = append(compressed, head...)
	compressed = append(compressed, providers.Message{
		Role: "user",
		Content: fmt.Sprintf("[CONTEXT COMPACTION — %d turns compressed]\n%s\n\nResolved: %s\nPending: %s",
			len(middle)/2, summary,
			strings.Join(resolved, "; "),
			strings.Join(pending, "; "),
		),
	})
	compressed = append(compressed, tail...)

	savings := float64(len(middle)) / float64(len(messages)) * 100
	return &CompressResultV3{
		Messages:         compressed,
		Summary:          summary,
		TurnsCompressed:  len(middle) / 2,
		OriginalTokens:   len(messages),
		CompressedTokens: len(compressed),
		SavingsPercent:   savings,
		ResolvedItems:    resolved,
		PendingItems:     pending,
	}, nil
}

func (c *ContextCompressorV3) extractTail(msgs []providers.Message, maxTokens int) []providers.Message {
	tokens := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		tokens += len(msgs[i].Content) / 4 // rough token estimate
		if tokens >= maxTokens {
			return msgs[i:]
		}
	}
	return msgs
}

func (c *ContextCompressorV3) pruneToolOutputs(msgs []providers.Message) []providers.Message {
	var pruned []providers.Message
	for _, msg := range msgs {
		if msg.Role == "tool" && len(msg.Content) > 2000 {
			msg.Content = msg.Content[:2000] + "\n... (tool output truncated)"
		}
		pruned = append(pruned, msg)
	}
	return pruned
}

func (c *ContextCompressorV3) summarizeV3(ctx context.Context, provider providers.Provider, model string, messages []providers.Message, maxTokens int) (string, []string, []string, error) {
	if provider == nil {
		return "", nil, nil, fmt.Errorf("no provider")
	}

	text := buildMessagesText(messages)
	prompt := fmt.Sprintf(`Summarize these conversation turns using the structured template below.

TEMPLATE:
## Summary
[2-3 sentences covering key decisions, files changed, and overall progress]

## Resolved
- [item that was completed]

## Pending
- [item still in progress]

## Key Decisions
- [architectural or design choice made]

## Files Changed
- [list with brief reason]

CONVERSATION:
%s`, text)

	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "system", Content: "You are a conversation summarizer. Follow the template exactly. Be concise."},
			{Role: "user", Content: prompt},
		},
		Model:   model,
		Options: map[string]any{"max_tokens": maxTokens},
	})
	if err != nil {
		return "", nil, nil, err
	}

	resolved := extractSection(resp.Content, "Resolved")
	pending := extractSection(resp.Content, "Pending")
	return resp.Content, resolved, pending, nil
}

func buildMessagesText(msgs []providers.Message) string {
	var sb strings.Builder
	for _, m := range msgs {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, truncateStr(m.Content, 300)))
	}
	return sb.String()
}

func extractSection(text, section string) []string {
	var items []string
	inSection := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## "+section) {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if inSection && strings.HasPrefix(line, "- ") {
			items = append(items, strings.TrimPrefix(line, "- "))
		}
	}
	return items
}
