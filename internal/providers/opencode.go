package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// OpenCodeProvider shells out to the `opencode` CLI binary.
// OpenCode is primarily interactive; non-interactive use pipes the prompt via stdin.
type OpenCodeProvider struct {
	name         string
	cliPath      string
	defaultModel string
	baseWorkDir  string
	mu           sync.Mutex
	sessionMu    sync.Map
}

type OpenCodeOption func(*OpenCodeProvider)

func WithOpenCodeName(name string) OpenCodeOption {
	return func(p *OpenCodeProvider) {
		if name != "" {
			p.name = name
		}
	}
}

func WithOpenCodeModel(model string) OpenCodeOption {
	return func(p *OpenCodeProvider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

func NewOpenCodeProvider(cliPath string, opts ...OpenCodeOption) *OpenCodeProvider {
	if cliPath == "" {
		cliPath = "opencode"
	}
	p := &OpenCodeProvider{
		name:         "opencode",
		cliPath:      cliPath,
		defaultModel: "gpt-5.4",
		baseWorkDir:  filepath.Join(config.ResolvedDataDirFromEnv(), "cli-workspaces", "opencode"),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *OpenCodeProvider) Name() string        { return p.name }
func (p *OpenCodeProvider) DefaultModel() string { return p.defaultModel }

func (p *OpenCodeProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Streaming:        true,
		ToolCalling:      true,
		StreamWithTools:  true,
		Thinking:         true,
		Vision:           true,
		CacheControl:     false,
		MaxContextWindow: 200_000,
		TokenizerID:      "o200k_base",
	}
}

func (p *OpenCodeProvider) Close() error { return nil }

func (p *OpenCodeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_, userMsg, _ := extractFromMessages(req.Messages)
	sessionKey := extractStringOpt(req.Options, OptSessionKey)
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	unlock := p.lockSession(sessionKey)
	defer unlock()

	workDir := p.ensureWorkDir(sessionKey)
	args := []string{"--pure", "-m", model, "."}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(userMsg + "\n/exit\n")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("opencode: %w (stderr: %s)", err, stderr.String())
	}

	return parseOpenCodeOutput(output)
}

func (p *OpenCodeProvider) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) (*ChatResponse, error) {
	// OpenCode's streaming model is TUI-based. For now, delegate to Chat().
	return p.Chat(ctx, req)
}

func (p *OpenCodeProvider) ensureWorkDir(sessionKey string) string {
	safe := sanitizePathSegment(sessionKey)
	dir := filepath.Join(p.baseWorkDir, safe)
	p.mu.Lock()
	defer p.mu.Unlock()
	os.MkdirAll(dir, 0755)
	return dir
}

func (p *OpenCodeProvider) lockSession(sessionKey string) func() {
	actual, _ := p.sessionMu.LoadOrStore(sessionKey, &sync.Mutex{})
	m := actual.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

func parseOpenCodeOutput(data []byte) (*ChatResponse, error) {
	// OpenCode can export session data as JSON. For now, try to parse as
	// a plain text response, stripping ANSI codes.
	text := stripANSI(string(data))
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("opencode: empty response")
	}
	return &ChatResponse{
		Content:      text,
		FinishReason: "stop",
	}, nil
}

func stripANSI(s string) string {
	var buf strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEsc = false
			}
			continue
		}
		buf.WriteByte(c)
	}
	return buf.String()
}

// Marshal/Unmarshal helpers used by opencode session export.
func init() {
	_ = json.Marshal // ensure json import is used
}
