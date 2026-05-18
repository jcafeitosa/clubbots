package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// CodexCLIProvider implements Provider by shelling out to the `codex` CLI binary.
// Follows the same subprocess pattern as ClaudeCLIProvider.
type CodexCLIProvider struct {
	name         string
	cliPath      string
	defaultModel string
	baseWorkDir  string
	permMode     string
	mu           sync.Mutex
	sessionMu    sync.Map
}

// CodexCLIOption configures the provider.
type CodexCLIOption func(*CodexCLIProvider)

// WithCodexCLIName overrides the provider name.
func WithCodexCLIName(name string) CodexCLIOption {
	return func(p *CodexCLIProvider) {
		if name != "" {
			p.name = name
		}
	}
}

// WithCodexCLIModel sets the default model.
func WithCodexCLIModel(model string) CodexCLIOption {
	return func(p *CodexCLIProvider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

// WithCodexCLIWorkDir sets the base work directory.
func WithCodexCLIWorkDir(dir string) CodexCLIOption {
	return func(p *CodexCLIProvider) {
		if dir != "" {
			p.baseWorkDir = dir
		}
	}
}

// NewCodexCLIProvider creates a provider that invokes the codex CLI.
func NewCodexCLIProvider(cliPath string, opts ...CodexCLIOption) *CodexCLIProvider {
	if cliPath == "" {
		cliPath = "codex"
	}
	p := &CodexCLIProvider{
		name:         "codex-cli",
		cliPath:      cliPath,
		defaultModel: "gpt-5.4",
		baseWorkDir:  filepath.Join(config.ResolvedDataDirFromEnv(), "cli-workspaces", "codex"),
		permMode:     "bypassPermissions",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *CodexCLIProvider) Name() string        { return p.name }
func (p *CodexCLIProvider) DefaultModel() string { return p.defaultModel }

// Capabilities returns the Codex CLI capability declaration.
func (p *CodexCLIProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Streaming:        true,
		ToolCalling:      true,
		StreamWithTools:  true,
		Thinking:         true,
		Vision:           true,
		CacheControl:     false,
		ImageGeneration:  true,
		MaxContextWindow: 1_000_000,
		TokenizerID:      "o200k_base",
	}
}

// Close is a no-op for CodexCLI (per-request subprocess, no persistent state).
func (p *CodexCLIProvider) Close() error { return nil }

// Chat runs the CLI synchronously and returns the response.
func (p *CodexCLIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	_, userMsg, _ := extractFromMessages(req.Messages)
	sessionKey := extractStringOpt(req.Options, OptSessionKey)
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	unlock := p.lockSession(sessionKey)
	defer unlock()

	workDir := p.ensureWorkDir(sessionKey)
	args := p.buildArgs(model, workDir, false)
	args = append(args, "--", userMsg)

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("codex-cli: %w (stderr: %s)", err, stderr.String())
	}

	return parseCodexCLIResponse(output)
}

// ChatStream runs the CLI with streaming output.
func (p *CodexCLIProvider) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) (*ChatResponse, error) {
	_, userMsg, _ := extractFromMessages(req.Messages)
	sessionKey := extractStringOpt(req.Options, OptSessionKey)
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	unlock := p.lockSession(sessionKey)
	defer unlock()

	workDir := p.ensureWorkDir(sessionKey)
	args := p.buildArgs(model, workDir, true)
	args = append(args, "--", userMsg)

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = workDir

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex-cli stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex-cli start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, StdioScanBufInit), StdioScanBufMax)

	var finalResp ChatResponse
	var contentBuf strings.Builder

	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ev cliStreamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "assistant":
			if ev.Message != nil {
				text, thinking := extractStreamContent(ev.Message)
				if text != "" {
					contentBuf.WriteString(text)
					onChunk(StreamChunk{Content: text})
				}
				if thinking != "" {
					onChunk(StreamChunk{Thinking: thinking})
				}
			}
		case "result":
			if ev.Result != "" {
				finalResp.Content = ev.Result
			} else {
				finalResp.Content = contentBuf.String()
			}
			finalResp.FinishReason = "stop"
			if ev.Subtype == "error" || ev.IsError {
				finalResp.FinishReason = "error"
			}
			if ev.Usage != nil {
				finalResp.Usage = &Usage{
					PromptTokens:     ev.Usage.InputTokens,
					CompletionTokens: ev.Usage.OutputTokens,
					TotalTokens:      ev.Usage.InputTokens + ev.Usage.OutputTokens,
				}
			}
		}
	}

	if ctx.Err() != nil {
		_ = cmd.Wait()
		return nil, ctx.Err()
	}

	if err := cmd.Wait(); err != nil {
		if finalResp.Content != "" {
			return &finalResp, nil
		}
		return nil, fmt.Errorf("codex-cli: %w (stderr: %s)", err, stderrBuf.String())
	}

	if finalResp.Content == "" {
		finalResp.Content = contentBuf.String()
		finalResp.FinishReason = "stop"
	}

	onChunk(StreamChunk{Done: true})
	return &finalResp, nil
}

func (p *CodexCLIProvider) buildArgs(model, workDir string, stream bool) []string {
	args := []string{
		"--model", model,
		"--permission-mode", p.permMode,
	}
	if stream {
		args = append(args, "--output-format", "stream-json")
	} else {
		args = append(args, "--output-format", "json")
	}
	return args
}

func (p *CodexCLIProvider) ensureWorkDir(sessionKey string) string {
	safe := sanitizePathSegment(sessionKey)
	dir := filepath.Join(p.baseWorkDir, safe)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("codex-cli: failed to create workdir", "dir", dir, "error", err)
		return os.TempDir()
	}
	return dir
}

func (p *CodexCLIProvider) lockSession(sessionKey string) func() {
	actual, _ := p.sessionMu.LoadOrStore(sessionKey, &sync.Mutex{})
	m := actual.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

func parseCodexCLIResponse(data []byte) (*ChatResponse, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("codex-cli: empty response")
	}

	var resp cliJSONResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return &ChatResponse{
			Content:      trimmed,
			FinishReason: "stop",
		}, nil
	}

	if resp.Type == "result" {
		cr := &ChatResponse{
			Content:      resp.Result,
			FinishReason: "stop",
		}
		if resp.Subtype == "error" {
			cr.FinishReason = "error"
		}
		return cr, nil
	}

	return &ChatResponse{
		Content:      trimmed,
		FinishReason: "stop",
	}, nil
}
