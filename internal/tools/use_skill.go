package tools

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/skills"
)

// UseSkillTool activates a skill by loading its SKILL.md content.
// The full skill content is returned in the tool result so the agent
// can follow the skill's instructions immediately — no separate read_file needed.
type UseSkillTool struct {
	loader *skills.Loader
}

func NewUseSkillTool(loader *skills.Loader) *UseSkillTool {
	return &UseSkillTool{loader: loader}
}

func (t *UseSkillTool) Name() string { return "use_skill" }

func (t *UseSkillTool) Description() string {
	return "Activate a skill by name. Returns the full skill instructions. Use this instead of read_file to load skill content — it is faster and injects the skill directly into context."
}

func (t *UseSkillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Skill name or slug to activate",
			},
		},
		"required": []string{"name"},
	}
}

func (t *UseSkillTool) Execute(ctx context.Context, args map[string]any) *Result {
	name, _ := args["name"].(string)
	if name == "" {
		return ErrorResult("name parameter is required")
	}

	if t.loader == nil {
		return ErrorResult("skill loader not available")
	}

	content, ok := t.loader.LoadSkill(ctx, name)
	if !ok {
		// Try search for suggestions
		allSkills := t.loader.ListSkills(ctx)
		var suggestions string
		for _, s := range allSkills {
			suggestions += fmt.Sprintf("\n- %s: %s", s.Name, s.Description)
		}
		if len(suggestions) > 0 {
			suggestions = "\n\nAvailable skills:" + suggestions
		}
		return ErrorResult(fmt.Sprintf("skill %q not found. Use skill_search to discover available skills.%s", name, suggestions))
	}

	slog.Info("skill.activated", "skill", name)

	result := fmt.Sprintf("[Skill loaded: %s]\n\n%s\n\nFollow the instructions above for this task.", name, content)
	return NewResult(result)
}
