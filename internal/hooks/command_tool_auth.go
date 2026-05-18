package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// CommandToolAuth reads command frontmatter allowed-tools declarations.
// Inspired by Claude Code plugins-official command pattern.
type CommandToolAuth struct {
	commandDir string
}

func NewCommandToolAuth(commandDir string) *CommandToolAuth {
	return &CommandToolAuth{commandDir: commandDir}
}

// CommandFrontmatter is the YAML frontmatter of a command .md file.
type CommandFrontmatter struct {
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	AllowedTools []string `json:"allowed-tools" yaml:"allowed-tools"`
}

// LoadCommands parses all command .md files and returns their frontmatter.
func (c *CommandToolAuth) LoadCommands() ([]CommandFrontmatter, error) {
	entries, err := os.ReadDir(c.commandDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var commands []CommandFrontmatter
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.commandDir, e.Name()))
		if err != nil {
			continue
		}
		fm, err := parseFrontmatter(data)
		if err != nil || fm.Name == "" {
			continue
		}
		commands = append(commands, fm)
	}
	return commands, nil
}

// AllowedToolsFor returns the pre-authorized tools for a command by name.
func (c *CommandToolAuth) AllowedToolsFor(commandName string) []string {
	cmds, _ := c.LoadCommands()
	for _, cmd := range cmds {
		if cmd.Name == commandName {
			return cmd.AllowedTools
		}
	}
	return nil
}

func parseFrontmatter(data []byte) (CommandFrontmatter, error) {
	content := string(data)
	var fm CommandFrontmatter

	if idx := strings.Index(content, "```json"); idx >= 0 {
		start := idx + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			if err := json.Unmarshal([]byte(content[start:start+end]), &fm); err == nil {
				return fm, nil
			}
		}
	}

	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]
		if end := strings.Index(rest, "\n---"); end > 0 {
			fm = parseSimpleYAML(rest[:end])
			return fm, nil
		}
	}

	return fm, nil
}

func parseSimpleYAML(yaml string) CommandFrontmatter {
	var fm CommandFrontmatter
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "name":
			fm.Name = val
		case "description":
			fm.Description = strings.Trim(val, `"`)
		case "allowed-tools":
			val = strings.Trim(val, "[] ")
			for _, t := range strings.Split(val, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					fm.AllowedTools = append(fm.AllowedTools, t)
				}
			}
		}
	}
	return fm
}
