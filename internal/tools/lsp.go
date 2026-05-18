package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// LSPTool provides symbol-level code intelligence via gopls and grep fallback.
// Inspired by Claude Code's LSP integration for large codebases.
type LSPTool struct{}

func NewLSPTool() *LSPTool { return &LSPTool{} }

func (t *LSPTool) Name() string        { return "lsp" }
func (t *LSPTool) Description() string { return symbolLevelDescription }

func (t *LSPTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: definition, references, hover, or symbols",
				"enum":        []string{"definition", "references", "hover", "symbols"},
			},
			"file": map[string]any{
				"type":        "string",
				"description": "Absolute or relative file path",
			},
			"line": map[string]any{
				"type":        "integer",
				"description": "Line number (1-based)",
			},
			"col": map[string]any{
				"type":        "integer",
				"description": "Column number (1-based, optional)",
			},
		},
		"required": []string{"action", "file", "line"},
	}
}

const symbolLevelDescription = `Symbol-level code intelligence via language servers.
Provides go-to-definition, find-references, and hover documentation.
Uses gopls for Go files, grep fallback for other languages.

ACTIONS:
- definition: Jump to symbol definition at file:line:col
- references: Find all references to symbol at file:line
- hover: Show surrounding context at file:line
- symbols: List functions/types in file (Go only)`

func (t *LSPTool) Execute(ctx context.Context, args map[string]any) *Result {
	action, _ := args["action"].(string)
	file, _ := args["file"].(string)
	line, _ := toInt(args["line"])
	col, _ := toInt(args["col"])

	if action == "" || file == "" {
		return &Result{ForLLM: "lsp: action and file are required", IsError: true}
	}

	var output string
	var err error
	switch action {
	case "definition":
		output, err = t.gotoDefinition(ctx, file, line, col)
	case "references":
		output, err = t.findReferences(ctx, file, line, col)
	case "hover":
		output, err = t.hover(ctx, file, line, col)
	case "symbols":
		output, err = t.documentSymbols(ctx, file)
	default:
		return &Result{ForLLM: fmt.Sprintf("lsp: unknown action %q (valid: definition, references, hover, symbols)", action), IsError: true}
	}

	if err != nil {
		return &Result{ForLLM: fmt.Sprintf("lsp: %s failed: %v", action, err), IsError: true}
	}
	if output == "" {
		return &Result{ForLLM: fmt.Sprintf("lsp: %s returned no results", action)}
	}
	return &Result{ForLLM: output}
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}

func (t *LSPTool) gotoDefinition(ctx context.Context, file string, line, col int) (string, error) {
	if strings.HasSuffix(file, ".go") {
		return t.goplsQuery(ctx, "definition", file, line, col)
	}
	return t.grepDefinition(ctx, file, line)
}

func (t *LSPTool) findReferences(ctx context.Context, file string, line, col int) (string, error) {
	if strings.HasSuffix(file, ".go") {
		return t.goplsQuery(ctx, "references", file, line, col)
	}
	return t.grepReferences(ctx, file, line)
}

func (t *LSPTool) hover(ctx context.Context, file string, line, col int) (string, error) {
	if strings.HasSuffix(file, ".go") {
		return t.goplsQuery(ctx, "hover", file, line, col)
	}
	return t.grepHover(ctx, file, line)
}

func (t *LSPTool) documentSymbols(ctx context.Context, file string) (string, error) {
	if strings.HasSuffix(file, ".go") {
		return t.goplsQuery(ctx, "symbols", file, 0, 0)
	}
	return "", fmt.Errorf("lsp: symbols action requires .go file (gopls)")
}

func (t *LSPTool) goplsQuery(ctx context.Context, action, file string, line, col int) (string, error) {
	var args []string
	switch action {
	case "references":
		args = []string{"references", file}
	default:
		args = []string{"query", file}
		if col > 0 {
			args = append(args, fmt.Sprintf(":#%d,%d", line, col))
		} else if line > 0 {
			args = append(args, fmt.Sprintf(":#%d", line))
		}
	}

	cmd := exec.CommandContext(ctx, "gopls", args...)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return "", fmt.Errorf("gopls %s: %w", action, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *LSPTool) grepDefinition(ctx context.Context, file string, line int) (string, error) {
	cmd := exec.CommandContext(ctx, "sed", "-n", fmt.Sprintf("%dp", line), file)
	sym, err := cmd.Output()
	if err != nil || len(sym) == 0 {
		return "", fmt.Errorf("cannot read line %d in %s", line, file)
	}

	word := extractSymbol(string(sym))
	if word == "" {
		return "", fmt.Errorf("no symbol found at %s:%d", file, line)
	}

	dir := file[:strings.LastIndex(file, "/")]
	cmd = exec.CommandContext(ctx, "grep", "-rn", fmt.Sprintf("func.*%s|type %s|def %s|class %s", word, word, word, word), dir)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("symbol %q not found", word)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *LSPTool) grepReferences(ctx context.Context, file string, line int) (string, error) {
	cmd := exec.CommandContext(ctx, "sed", "-n", fmt.Sprintf("%dp", line), file)
	sym, err := cmd.Output()
	if err != nil {
		return "", err
	}
	word := extractSymbol(string(sym))
	if word == "" {
		return "", fmt.Errorf("no symbol at %s:%d", file, line)
	}
	dir := file[:strings.LastIndex(file, "/")]
	cmd = exec.CommandContext(ctx, "grep", "-rn", word, dir)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *LSPTool) grepHover(ctx context.Context, file string, line int) (string, error) {
	cmd := exec.CommandContext(ctx, "sed", "-n", fmt.Sprintf("%d,%dp", line-2, line+5), file)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func extractSymbol(line string) string {
	line = strings.TrimSpace(line)
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_')
	})
	for _, f := range fields {
		if len(f) > 1 && !isKeyword(f) {
			return f
		}
	}
	return ""
}

func isKeyword(s string) bool {
	return map[string]bool{
		"func": true, "type": true, "var": true, "const": true, "import": true,
		"return": true, "if": true, "else": true, "for": true, "range": true,
		"switch": true, "case": true, "default": true, "break": true, "continue": true,
		"def": true, "class": true, "public": true, "private": true, "protected": true,
		"static": true, "void": true, "int": true, "string": true, "bool": true,
	}[s]
}
