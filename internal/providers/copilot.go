package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// CopilotProvider implements Provider by shelling out to the `copilot` CLI binary.
type CopilotProvider struct {
	name         string
	cliPath      string
	defaultModel string
	baseWorkDir  string
	permMode     string
	mu           sync.Mutex
	sessionMu    sync.Map
}

// CopilotOption configures the provider.
type CopilotOption func(*CopilotProvider)

// WithCopilotName overrides the provider name.
func WithCopilotName(name string) CopilotOption {
	return func(p *CopilotProvider) {
		if name != "" {
			p.name = name
		}
	}
}

// WithCopilotModel sets the default model.
func WithCopilotModel(model string) CopilotOption {
	return func(p *CopilotProvider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

// NewCopilotProvider creates a provider for the GitHub Copilot CLI.
func NewCopilotProvider(cliPath string, opts ...CopilotOption) *CopilotProvider {
	if cliPath == "" {
		cliPath = "copilot"
	}
	p := &CopilotProvider{
		name:         "copilot",
		cliPath:      cliPath,
		defaultModel: "gpt-5.4",
		baseWorkDir:  filepath.Join(config.ResolvedDataDirFromEnv(), "cli-workspaces", "copilot"),
		permMode:     "bypassPermissions",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *CopilotProvider) Name() string        { return p.name }
func (p *CopilotProvider) DefaultModel() string { return p.defaultModel }

func (p *CopilotProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Streaming:        true,
		ToolCalling:      true,
		StreamWithTools:  true,
		Thinking:         true,
		Vision:           true,
		CacheControl:     false,
		MaxContextWindow: 1_000_000,
		TokenizerID:      "o200k_base",
	}
}

func (p *CopilotProvider) Close() error { return nil }

// Chat runs the CLI synchronously.
func (p *CopilotProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
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
		return nil, fmt.Errorf("copilot: %w (stderr: %s)", err, stderr.String())
	}

	return parseCLIJSONResponse(output)
}

// ChatStream runs the CLI with streaming output.
func (p *CopilotProvider) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) (*ChatResponse, error) {
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
		return nil, fmt.Errorf("copilot stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("copilot start: %w", err)
	}

	return scanCLIStream(ctx, cmd, stdout, &stderrBuf, onChunk, "copilot")
}

func (p *CopilotProvider) buildArgs(model string, stream bool) []string {
	args := []string{"--model", model}
	if stream {
		args = append(args, "--output-format", "stream-json")
	} else {
		args = append(args, "--output-format", "json")
	}
	return args
}

func (p *CopilotProvider) ensureWorkDir(sessionKey string) string {
	safe := sanitizePathSegment(sessionKey)
	dir := filepath.Join(p.baseWorkDir, safe)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("copilot: failed to create workdir", "dir", dir, "error", err)
		return os.TempDir()
	}
	return dir
}

func (p *CopilotProvider) lockSession(sessionKey string) func() {
	actual, _ := p.sessionMu.LoadOrStore(sessionKey, &sync.Mutex{})
	m := actual.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// scanCLIStream is a shared streaming line scanner for CLI providers.
func scanCLIStream(ctx context.Context, cmd *exec.Cmd, stdout io.Reader, stderr *bytes.Buffer, onChunk func(StreamChunk), name string) (*ChatResponse, error) {
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
		return nil, fmt.Errorf("%s: %w (stderr: %s)", name, err, stderr.String())
	}

	if finalResp.Content == "" {
		finalResp.Content = contentBuf.String()
		finalResp.FinishReason = "stop"
	}

	onChunk(StreamChunk{Done: true})
	return &finalResp, nil
}

// parseCLIJSONResponse parses CLI JSON output into a ChatResponse.
func parseCLIJSONResponse(data []byte) (*ChatResponse, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("cli: empty response")
	}

	var resp cliJSONResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return &ChatResponse{Content: trimmed, FinishReason: "stop"}, nil
	}

	if resp.Type == "result" {
		cr := &ChatResponse{Content: resp.Result, FinishReason: "stop"}
		if resp.Subtype == "error" {
			cr.FinishReason = "error"
		}
		return cr, nil
	}

	return &ChatResponse{Content: trimmed, FinishReason: "stop"}, nil
}
