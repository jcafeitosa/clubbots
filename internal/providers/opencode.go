package providers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// OpenCodeProvider implements Provider by shelling out to the `opencode` CLI binary.
type OpenCodeProvider struct {
	name         string
	cliPath      string
	defaultModel string
	baseWorkDir  string
	permMode     string
	mu           sync.Mutex
	sessionMu    sync.Map
}

// OpenCodeOption configures the provider.
type OpenCodeOption func(*OpenCodeProvider)

// WithOpenCodeName overrides the provider name.
func WithOpenCodeName(name string) OpenCodeOption {
	return func(p *OpenCodeProvider) {
		if name != "" {
			p.name = name
		}
	}
}

// WithOpenCodeModel sets the default model.
func WithOpenCodeModel(model string) OpenCodeOption {
	return func(p *OpenCodeProvider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

// NewOpenCodeProvider creates a provider for the OpenCode CLI.
func NewOpenCodeProvider(cliPath string, opts ...OpenCodeOption) *OpenCodeProvider {
	if cliPath == "" {
		cliPath = "opencode"
	}
	p := &OpenCodeProvider{
		name:         "opencode",
		cliPath:      cliPath,
		defaultModel: "gpt-5.4",
		baseWorkDir:  filepath.Join(config.ResolvedDataDirFromEnv(), "cli-workspaces", "opencode"),
		permMode:     "bypassPermissions",
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

// Chat runs the CLI synchronously.
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
	args := p.buildArgs(model, false)
	args = append(args, "--", userMsg)

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("opencode: %w (stderr: %s)", err, stderr.String())
	}

	return parseCLIJSONResponse(output)
}

// ChatStream runs the CLI with streaming output.
func (p *OpenCodeProvider) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) (*ChatResponse, error) {
	_, userMsg, _ := extractFromMessages(req.Messages)
	sessionKey := extractStringOpt(req.Options, OptSessionKey)
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	unlock := p.lockSession(sessionKey)
	defer unlock()

	workDir := p.ensureWorkDir(sessionKey)
	args := p.buildArgs(model, true)
	args = append(args, "--", userMsg)

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = workDir

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("opencode stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("opencode start: %w", err)
	}

	return scanCLIStream(ctx, cmd, stdout, &stderrBuf, onChunk, "opencode")
}

func (p *OpenCodeProvider) buildArgs(model string, stream bool) []string {
	args := []string{"--model", model}
	if stream {
		args = append(args, "--output-format", "stream-json")
	} else {
		args = append(args, "--output-format", "json")
	}
	return args
}

func (p *OpenCodeProvider) ensureWorkDir(sessionKey string) string {
	safe := sanitizePathSegment(sessionKey)
	dir := filepath.Join(p.baseWorkDir, safe)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("opencode: failed to create workdir", "dir", dir, "error", err)
		return os.TempDir()
	}
	return dir
}

func (p *OpenCodeProvider) lockSession(sessionKey string) func() {
	actual, _ := p.sessionMu.LoadOrStore(sessionKey, &sync.Mutex{})
	m := actual.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}
