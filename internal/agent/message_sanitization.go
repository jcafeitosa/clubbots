package agent

import (
	"strings"
	"unicode/utf8"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// SanitizeMessages cleans messages before sending to the LLM.
// Inspired by Hermes Agent's message_sanitization.py.
// Handles: surrogate characters, non-ASCII repair, image stripping, empty content.

// SanitizeMessagesForProvider applies provider-specific sanitization.
func SanitizeMessagesForProvider(msgs []providers.Message, providerName string) []providers.Message {
	cleaned := make([]providers.Message, 0, len(msgs))

	for _, msg := range msgs {
		// Strip images if provider doesn't support vision
		if !providerSupportsVision(providerName) && len(msg.Images) > 0 {
			msg.Images = nil
		}

		// Clean content
		msg.Content = sanitizeContent(msg.Content)

		// Skip empty messages (except tool results which may be empty)
		if msg.Content == "" && msg.Role != "tool" {
			continue
		}

		cleaned = append(cleaned, msg)
	}

	return cleaned
}

// sanitizeContent cleans a message content string.
func sanitizeContent(content string) string {
	// 1. Remove surrogate characters (invalid UTF-16 surrogates)
	content = removeSurrogates(content)

	// 2. Ensure valid UTF-8
	if !utf8.ValidString(content) {
		content = strings.ToValidUTF8(content, "")
	}

	// 3. Normalize whitespace (no more than 2 consecutive newlines)
	content = normalizeWhitespace(content)

	// 4. Trim
	content = strings.TrimSpace(content)

	return content
}

// removeSurrogates strips Unicode surrogate characters (U+D800-U+DFFF).
func removeSurrogates(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	for _, r := range s {
		if r < 0xD800 || r > 0xDFFF {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// normalizeWhitespace collapses 3+ newlines into 2.
func normalizeWhitespace(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// StripImagesFromMessages removes all image content blocks.
func StripImagesFromMessages(msgs []providers.Message) []providers.Message {
	for i := range msgs {
		msgs[i].Images = nil
	}
	return msgs
}

// providerSupportsVision returns true if the provider supports image inputs.
func providerSupportsVision(name string) bool {
	switch name {
	case "anthropic", "anthropic_native", "openai", "openai_compat",
		"gemini", "gemini_native", "openrouter", "codex-cli",
		"copilot", "opencode", "codex":
		return true
	}
	return false
}

// SanitizeToolsNonASCII ensures tool definitions contain only ASCII characters.
// Some providers (e.g., llama.cpp) reject non-ASCII in tool schemas.
func SanitizeToolsNonASCII(tools []providers.ToolDefinition) []providers.ToolDefinition {
	for i := range tools {
		if tools[i].Function != nil {
			tools[i].Function.Name = sanitizeASCII(tools[i].Function.Name)
			tools[i].Function.Description = sanitizeASCII(tools[i].Function.Description)
		}
	}
	return tools
}

func sanitizeASCII(s string) string {
	var result strings.Builder
	for _, r := range s {
		if r < 128 {
			result.WriteRune(r)
		}
	}
	return result.String()
}
