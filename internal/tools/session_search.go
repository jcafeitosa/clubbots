package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// SessionSearchTool provides conversation recall across sessions.
// Inspired by Hermes Agent's session_search_tool.py — three modes.
type SessionSearchTool struct {
	sessions store.SessionStore
}

func NewSessionSearchTool(s store.SessionStore) *SessionSearchTool {
	return &SessionSearchTool{sessions: s}
}

func (t *SessionSearchTool) Name() string        { return "session_search" }
func (t *SessionSearchTool) Description() string { return sessionSearchDescription }

func (t *SessionSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":        "string",
				"description": "Search mode: discover (keyword search), scroll (paginate results), browse (list recent)",
				"enum":        []string{"discover", "scroll", "browse"},
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (for discover mode)",
			},
			"agentId": map[string]any{
				"type":        "string",
				"description": "Filter by agent ID",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max results (default 10)",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Pagination offset (for scroll mode)",
			},
		},
		"required": []string{"mode"},
	}
}

const sessionSearchDescription = `Search and browse past conversation sessions.
Three modes:
- discover: keyword search across all session messages
- scroll: paginate through search results
- browse: list recent sessions for the current agent`

func (t *SessionSearchTool) Execute(ctx context.Context, args map[string]any) *Result {
	mode, _ := args["mode"].(string)
	query, _ := args["query"].(string)
	agentID, _ := args["agentId"].(string)
	limit := toIntDefault(args["limit"], 10)
	offset := toIntDefault(args["offset"], 0)

	if t.sessions == nil {
		return &Result{ForLLM: "session_search: no session store available", IsError: true}
	}

	switch mode {
	case "discover":
		return t.discover(ctx, query, agentID, limit)
	case "scroll":
		return t.scroll(ctx, agentID, limit, offset)
	case "browse":
		return t.browse(ctx, agentID, limit)
	default:
		return &Result{ForLLM: fmt.Sprintf("session_search: unknown mode %q (valid: discover, scroll, browse)", mode), IsError: true}
	}
}

func (t *SessionSearchTool) discover(ctx context.Context, query, agentID string, limit int) *Result {
	opts := store.SessionListOpts{
		AgentID: agentID,
		Limit:   limit * 3, // fetch more for filtering
	}
	result := t.sessions.ListPagedRich(ctx, opts)

	var matches []string
	for _, s := range result.Sessions {
		if query == "" || sessionContains(s, query) {
			entry := fmtSessionEntry(s)
			matches = append(matches, entry)
			if len(matches) >= limit {
				break
			}
		}
	}

	if len(matches) == 0 {
		return &Result{ForLLM: fmt.Sprintf("No sessions found matching %q", query)}
	}

	return &Result{ForLLM: fmt.Sprintf("Found %d sessions:\n%s", len(matches), strings.Join(matches, "\n"))}
}

func (t *SessionSearchTool) scroll(ctx context.Context, agentID string, limit, offset int) *Result {
	opts := store.SessionListOpts{
		AgentID: agentID,
		Limit:   limit,
		Offset:  offset,
	}
	result := t.sessions.ListPagedRich(ctx, opts)

	var entries []string
	for _, s := range result.Sessions {
		entries = append(entries, fmtSessionEntry(s))
	}

	return &Result{ForLLM: fmt.Sprintf("Sessions %d-%d of %d:\n%s", offset, offset+len(result.Sessions), result.Total, strings.Join(entries, "\n"))}
}

func (t *SessionSearchTool) browse(ctx context.Context, agentID string, limit int) *Result {
	return t.scroll(ctx, agentID, limit, 0)
}

func fmtSessionEntry(s store.SessionInfoRich) string {
	label := s.Label
	if label == "" {
		label = "(untitled)"
	}
	return fmt.Sprintf("- %s | %s | %d msgs | %s | %s",
		s.Key, label, s.MessageCount, s.Model, s.Updated.Format("Jan 2 15:04"))
}

func sessionContains(s store.SessionInfoRich, query string) bool {
	ql := strings.ToLower(query)
	if strings.Contains(strings.ToLower(s.Label), ql) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Key), ql) {
		return true
	}
	return false
}

func toIntDefault(v any, defaultVal int) int {
	if v == nil {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return defaultVal
}
