package providers

import "slices"

import "strings"

// ReasoningCapability describes the supported reasoning levels for a model.
// The registry is intentionally narrow: only GPT-5/Codex families we can
// validate reliably today get explicit entries.
type ReasoningCapability struct {
	Levels        []string `json:"levels,omitempty"`
	DefaultEffort string   `json:"default_effort,omitempty"`
}

func (c *ReasoningCapability) Supports(level string) bool {
	if c == nil || level == "" {
		return false
	}
	return slices.Contains(c.Levels, level)
}

type reasoningCapabilityEntry struct {
	id         string
	capability ReasoningCapability
}

var reasoningCapabilityEntries = []reasoningCapabilityEntry{
	{id: "gpt-5.4-mini", capability: ReasoningCapability{Levels: []string{"none", "low", "medium", "high", "xhigh"}, DefaultEffort: "none"}},
	{id: "gpt-5-mini", capability: ReasoningCapability{Levels: []string{"none", "low", "medium", "high", "xhigh"}, DefaultEffort: "none"}},
	{id: "gpt-5.4", capability: ReasoningCapability{Levels: []string{"none", "low", "medium", "high", "xhigh"}, DefaultEffort: "none"}},
	{id: "gpt-5.3-codex-spark", capability: ReasoningCapability{Levels: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"}},
	{id: "gpt-5.3-codex", capability: ReasoningCapability{Levels: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"}},
	{id: "gpt-5.2-codex", capability: ReasoningCapability{Levels: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"}},
	{id: "gpt-5.2", capability: ReasoningCapability{Levels: []string{"none", "low", "medium", "high", "xhigh"}, DefaultEffort: "none"}},
	{id: "gpt-5.1-codex-max", capability: ReasoningCapability{Levels: []string{"none", "medium", "high", "xhigh"}, DefaultEffort: "none"}},
	{id: "gpt-5.1-codex-mini", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "gpt-5.1-codex", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "gpt-5.1", capability: ReasoningCapability{Levels: []string{"none", "low", "medium", "high"}, DefaultEffort: "none"}},
	{id: "gpt-5-codex-mini", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "gpt-5-codex", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "gpt-5", capability: ReasoningCapability{Levels: []string{"minimal", "low", "medium", "high"}, DefaultEffort: "medium"}},

	// Anthropic extended thinking (enable_thinking + thinking_budget tokens)
	{id: "claude-opus-4-7", capability: ReasoningCapability{Levels: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"}},
	{id: "claude-opus-4-6", capability: ReasoningCapability{Levels: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"}},
	{id: "claude-sonnet-4-6", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "claude-sonnet-4-5", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "claude-haiku-4-5", capability: ReasoningCapability{Levels: []string{"low", "medium"}, DefaultEffort: "low"}},

	// Gemini thinking models
	{id: "gemini-3.0-flash", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "gemini-3.0-pro", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "gemini-2.5-pro", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "gemini-2.5-flash", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},

	// DeepSeek reasoner
	{id: "deepseek-reasoner", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "deepseek-chat", capability: ReasoningCapability{Levels: []string{"none", "low", "medium"}, DefaultEffort: "low"}},

	// Qwen3 thinking models (DashScope/Bailian)
	{id: "qwen3.6-plus", capability: ReasoningCapability{Levels: []string{"low", "medium", "high", "xhigh"}, DefaultEffort: "medium"}},
	{id: "qwen3.5-plus", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "qwen3.5-flash", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "low"}},
	{id: "qwen3.5-turbo", capability: ReasoningCapability{Levels: []string{"low", "medium"}, DefaultEffort: "low"}},
	{id: "qwen3-max", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "qwen3-plus", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "qwen3-turbo", capability: ReasoningCapability{Levels: []string{"low", "medium"}, DefaultEffort: "low"}},

	// Groq reasoning models
	{id: "deepseek-r1-distill-llama-70b", capability: ReasoningCapability{Levels: []string{"low", "medium", "high"}, DefaultEffort: "medium"}},
	{id: "llama-4-maverick", capability: ReasoningCapability{Levels: []string{"low", "medium"}, DefaultEffort: "low"}},
}

func LookupReasoningCapability(model string) *ReasoningCapability {
	normalized := normalizeReasoningModel(model)
	if normalized == "" {
		return nil
	}
	for _, entry := range reasoningCapabilityEntries {
		if normalized == entry.id {
			capability := entry.capability
			capability.Levels = append([]string(nil), capability.Levels...)
			return &capability
		}
	}
	return nil
}

func normalizeReasoningModel(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return ""
	}
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		normalized = normalized[idx+1:]
	}
	return normalized
}
