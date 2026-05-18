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
		return t.lspQuery(ctx, "definition", file, line, col)
	}
	return t.grepDefinition(ctx, file, line)
}

func (t *LSPTool) findReferences(ctx context.Context, file string, line, col int) (string, error) {
	if strings.HasSuffix(file, ".go") {
		return t.lspQuery(ctx, "references", file, line, col)
	}
	return t.grepReferences(ctx, file, line)
}

func (t *LSPTool) hover(ctx context.Context, file string, line, col int) (string, error) {
	if strings.HasSuffix(file, ".go") {
		return t.lspQuery(ctx, "hover", file, line, col)
	}
	return t.grepHover(ctx, file, line)
}

func (t *LSPTool) documentSymbols(ctx context.Context, file string) (string, error) {
	return t.lspQuery(ctx, "symbols", file, 0, 0)
}

// lspConfig maps file extensions to language server commands.
var lspConfig = map[string]struct {
	binary string
	query  []string // args template for "query" action (use {file}, {line}, {col})
	refs   []string // args template for "references"
	syms   []string // args template for "symbols"
}{
	".go":   {binary: "gopls", query: []string{"query", "{file}", ":#{line},{col}"}, refs: []string{"references", "{file}"}, syms: []string{"query", "{file}", ":#0,0"}},
	".ts":   {binary: "typescript-language-server", query: []string{"--stdio"}, refs: nil, syms: nil},
	".tsx":  {binary: "typescript-language-server", query: []string{"--stdio"}, refs: nil, syms: nil},
	".js":   {binary: "typescript-language-server", query: []string{"--stdio"}, refs: nil, syms: nil},
	".py":   {binary: "pyright", query: []string{"--stdio"}, refs: nil, syms: nil},
	".rs":   {binary: "rust-analyzer", query: []string{"--stdio"}, refs: nil, syms: nil},
}

func (t *LSPTool) lspQuery(ctx context.Context, action, file string, line, col int) (string, error) {
	ext := file[strings.LastIndex(file, "."):]
	cfg, ok := lspConfig[ext]
	if !ok {
		return "", fmt.Errorf("no language server configured for %s files", ext)
	}

	switch action {
	case "references":
		if cfg.refs == nil {
			return t.grepReferences(ctx, file, line)
		}
		return t.runLSP(ctx, cfg.binary, cfg.refs, file, line, col)
	case "symbols":
		if cfg.syms == nil {
			return t.grepSymbols(ctx, file)
		}
		return t.runLSP(ctx, cfg.binary, cfg.syms, file, line, col)
	default:
		if cfg.query == nil {
			return t.grepDefinition(ctx, file, line)
		}
		return t.runLSP(ctx, cfg.binary, cfg.query, file, line, col)
	}
}

func (t *LSPTool) runLSP(ctx context.Context, binary string, tmpl []string, file string, line, col int) (string, error) {
	if _, err := exec.LookPath(binary); err != nil {
		return "", fmt.Errorf("language server %q not installed (try: brew install %s)", binary, binary)
	}
	args := make([]string, len(tmpl))
	for i, a := range tmpl {
		a = strings.ReplaceAll(a, "{file}", file)
		a = strings.ReplaceAll(a, "{line}", fmt.Sprintf("%d", line))
		a = strings.ReplaceAll(a, "{col}", fmt.Sprintf("%d", col))
		args[i] = a
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return "", fmt.Errorf("%s %s: %w", binary, args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (t *LSPTool) grepSymbols(ctx context.Context, file string) (string, error) {
	ext := file[strings.LastIndex(file, "."):]
	var pattern string
	switch ext {
	case ".go": pattern = `^func |^type `
	case ".py": pattern = `^def |^class `
	case ".ts", ".tsx", ".js": pattern = `^export (function|class|const|interface|type) `
	case ".rs": pattern = `^pub (fn|struct|enum|trait|impl) `
	default: pattern = `^func |^def |^class `
	}
	cmd := exec.CommandContext(ctx, "grep", "-n", pattern, file)
	out, err := cmd.Output()
	if err != nil {
		return "", err
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
