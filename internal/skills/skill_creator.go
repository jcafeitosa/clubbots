package skills

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// SkillCreator enables agents to create and improve skills autonomously.
// Inspired by both Claude Code's SKILL.md system and Hermes Agent's self-improvement loop.
type SkillCreator struct {
	skillsDir string
	provider  providers.Provider
	model     string
}

// NewSkillCreator creates a skill creator that writes to the given directory.
func NewSkillCreator(skillsDir string, p providers.Provider, model string) *SkillCreator {
	os.MkdirAll(skillsDir, 0755)
	return &SkillCreator{skillsDir: skillsDir, provider: p, model: model}
}

// SkillFrontmatter is the YAML frontmatter for SKILL.md files.
// Based on agentskills.io spec compatible with Claude Code and Hermes Agent.
type SkillFrontmatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Version     string            `yaml:"version"`
	Tags        []string          `yaml:"tags,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// CreateSkill generates a new SKILL.md from agent-discovered patterns.
// The agent calls this when it finds a repeatable workflow worth capturing.
func (c *SkillCreator) CreateSkill(ctx context.Context, slug string, fm SkillFrontmatter, body string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", err
	}

	dir := filepath.Join(c.skillsDir, slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("skill_creator: %w", err)
	}

	content := buildSkillMarkdown(fm, body)
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("skill_creator: write: %w", err)
	}

	slog.Info("skill_creator: created skill",
		"slug", slug,
		"name", fm.Name,
		"path", path,
	)
	return path, nil
}

// ImproveSkill refines an existing skill based on agent feedback.
// Reads the current SKILL.md, asks the LLM to improve it, writes back.
func (c *SkillCreator) ImproveSkill(ctx context.Context, slug, feedback string) error {
	if err := validateSlug(slug); err != nil {
		return err
	}

	path := filepath.Join(c.skillsDir, slug, "SKILL.md")
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("skill_creator: read: %w", err)
	}

	improved, err := c.llmImprove(ctx, string(current), feedback)
	if err != nil {
		return fmt.Errorf("skill_creator: improve: %w", err)
	}

	if err := os.WriteFile(path, []byte(improved), 0644); err != nil {
		return fmt.Errorf("skill_creator: write: %w", err)
	}

	slog.Info("skill_creator: improved skill", "slug", slug)
	return nil
}

// DiscoverPatterns analyzes recent agent turns to find repeatable workflows.
// Returns suggested skills that could be created.
func (c *SkillCreator) DiscoverPatterns(ctx context.Context, turns []TurnSummary) ([]SkillSuggestion, error) {
	if c.provider == nil {
		return nil, nil
	}

	prompt := buildDiscoveryPrompt(turns)
	resp, err := c.provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "system", Content: patternDiscoveryPrompt},
			{Role: "user", Content: prompt},
		},
		Model:   c.model,
		Options: map[string]any{"max_tokens": 1024},
	})
	if err != nil {
		return nil, err
	}

	return parseSuggestions(resp.Content), nil
}

// TurnSummary captures a turn for pattern discovery.
type TurnSummary struct {
	UserGoal      string   `json:"user_goal"`
	ToolsUsed     []string `json:"tools_used"`
	FilesChanged  []string `json:"files_changed"`
	Success       bool     `json:"success"`
	DurationMs    int64    `json:"duration_ms"`
}

// SkillSuggestion is a proposed new skill from pattern discovery.
type SkillSuggestion struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"` // 0.0-1.0
	Body        string  `json:"body"`
}

func (c *SkillCreator) llmImprove(ctx context.Context, current, feedback string) (string, error) {
	if c.provider == nil {
		return current, nil
	}
	resp, err := c.provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "system", Content: "You are a skill editor. Improve the SKILL.md based on feedback. Preserve the YAML frontmatter format. Return the complete improved SKILL.md."},
			{Role: "user", Content: fmt.Sprintf("CURRENT SKILL.md:\n%s\n\nFEEDBACK:\n%s\n\nReturn the improved SKILL.md:", current, feedback)},
		},
		Model:   c.model,
		Options: map[string]any{"max_tokens": 2048},
	})
	if err != nil {
		return current, err
	}
	return resp.Content, nil
}

func buildSkillMarkdown(fm SkillFrontmatter, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", fm.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", fm.Description))
	if fm.Version != "" {
		sb.WriteString(fmt.Sprintf("version: %s\n", fm.Version))
	}
	if len(fm.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(fm.Tags, ", ")))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	return sb.String()
}

const patternDiscoveryPrompt = `Analyze these agent turns and identify repeatable workflows worth capturing as skills.
A skill is worth creating when:
1. The same tool combination appears 3+ times
2. A multi-step workflow succeeded and could be reused
3. A debugging pattern was effective

For each suggestion, provide:
- name: short hyphenated name (max 64 chars)
- description: when to use this skill (max 1024 chars)
- confidence: 0.0-1.0
- body: the SKILL.md body with step-by-step instructions

Respond with JSON array. Be conservative — only suggest genuinely repeatable patterns.`

func buildDiscoveryPrompt(turns []TurnSummary) string {
	var sb strings.Builder
	for i, t := range turns {
		sb.WriteString(fmt.Sprintf("Turn %d: goal=%q tools=%v success=%v\n",
			i+1, t.UserGoal, t.ToolsUsed, t.Success))
	}
	return sb.String()
}

func parseSuggestions(content string) []SkillSuggestion {
	var suggestions []SkillSuggestion
	// Simple heuristic: look for named patterns in the response
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			suggestions = append(suggestions, SkillSuggestion{
				Description: strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "),
				Confidence:  0.5,
			})
		}
	}
	return suggestions
}

func validateSlug(slug string) error {
	if slug == "" || len(slug) > 64 {
		return fmt.Errorf("skill_creator: slug must be 1-64 chars")
	}
	for _, c := range slug {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("skill_creator: slug contains invalid char: %c", c)
		}
	}
	return nil
}

// RecordUsage updates skill usage telemetry for lifecycle tracking.
func RecordUsage(skillsDir, slug string) {
	usageFile := filepath.Join(skillsDir, ".usage.json")
	now := time.Now().UTC().Format(time.RFC3339)
	usage := fmt.Sprintf(`{"%s": {"last_used": "%s"}}`, slug, now)
	os.WriteFile(usageFile, []byte(usage), 0644)
}
