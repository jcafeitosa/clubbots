package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// OpenCodeProvider shells out to `opencode run` for non-interactive chat.
// OpenCode also supports ACP natively (opencode acp); the gateway uses ACPProvider
// for full protocol support. This provider is a lightweight alternative using `run`.
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
		defaultModel: "github-copilot/claude-sonnet-4.5",
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

	workspace := extractStringOpt(req.Options, OptWorkspace)
	workDir := p.ensureWorkDir(sessionKey, workspace)
	args := []string{"run", userMsg, "--model", model, "--format", "json", "--dangerously-skip-permissions", "--dir", workDir}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = nil

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("opencode: %w (stderr: %s)", err, stderr.String())
	}

	return parseOpenCodeJSONL(output)
}

func (p *OpenCodeProvider) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) (*ChatResponse, error) {
	_, userMsg, _ := extractFromMessages(req.Messages)
	sessionKey := extractStringOpt(req.Options, OptSessionKey)
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	unlock := p.lockSession(sessionKey)
	defer unlock()

	workspace := extractStringOpt(req.Options, OptWorkspace)
	workDir := p.ensureWorkDir(sessionKey, workspace)
	args := []string{"run", userMsg, "--model", model, "--format", "json", "--dangerously-skip-permissions", "--dir", workDir}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = workDir

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("opencode stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("opencode start: %w", err)
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

		var ev openCodeJSONLEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "text":
			if ev.Part != nil && ev.Part.Text != "" {
				contentBuf.WriteString(ev.Part.Text)
				onChunk(StreamChunk{Content: ev.Part.Text})
			}
		case "step_finish":
			finalResp.Content = contentBuf.String()
			finalResp.FinishReason = "stop"
			if ev.Part != nil && ev.Part.Tokens != nil {
				finalResp.Usage = &Usage{
					PromptTokens:     ev.Part.Tokens.Input,
					CompletionTokens: ev.Part.Tokens.Output,
					TotalTokens:      ev.Part.Tokens.Total,
				}
			}
		case "error":
			if finalResp.Content == "" && ev.Error != nil && ev.Error.Data != nil {
				finalResp.Content = ev.Error.Data.Message
				finalResp.FinishReason = "error"
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
		return nil, fmt.Errorf("opencode: %w (stderr: %s)", err, stderrBuf.String())
	}

	if finalResp.Content == "" {
		finalResp.Content = contentBuf.String()
		finalResp.FinishReason = "stop"
	}

	onChunk(StreamChunk{Done: true})
	return &finalResp, nil
}

func (p *OpenCodeProvider) ensureWorkDir(sessionKey, workspace string) string {
	// If a workspace is provided and exists, use it directly (OpenCode needs real directory context).
	if workspace != "" {
		if fi, err := os.Stat(workspace); err == nil && fi.IsDir() {
			return workspace
		}
	}
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

// openCodeJSONLEvent represents a single JSONL line from `opencode run --format json`.
type openCodeJSONLEvent struct {
	Type      string           `json:"type"`
	Part      *openCodePart    `json:"part,omitempty"`
	Error     *openCodeErr     `json:"error,omitempty"`
	SessionID string           `json:"sessionID"`
}

type openCodePart struct {
	Type   string         `json:"type"`
	Text   string         `json:"text"`
	Tokens *openCodeTokens `json:"tokens,omitempty"`
}

type openCodeTokens struct {
	Total  int `json:"total"`
	Input  int `json:"input"`
	Output int `json:"output"`
}

type openCodeErr struct {
	Data *openCodeErrData `json:"data,omitempty"`
}

type openCodeErrData struct {
	Message string `json:"message"`
}

func parseOpenCodeJSONL(data []byte) (*ChatResponse, error) {
	lines := bytes.Split(data, []byte("\n"))
	var content strings.Builder
	var usage *Usage
	var errMsg string

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev openCodeJSONLEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "text":
			if ev.Part != nil && ev.Part.Text != "" {
				content.WriteString(ev.Part.Text)
			}
		case "step_finish":
			if ev.Part != nil && ev.Part.Tokens != nil {
				usage = &Usage{
					PromptTokens:     ev.Part.Tokens.Input,
					CompletionTokens: ev.Part.Tokens.Output,
					TotalTokens:      ev.Part.Tokens.Total,
				}
			}
		case "error":
			if ev.Error != nil && ev.Error.Data != nil {
				errMsg = ev.Error.Data.Message
			}
		}
	}

	if content.Len() == 0 && errMsg != "" {
		return &ChatResponse{
			Content:      errMsg,
			FinishReason: "error",
		}, nil
	}

	if content.Len() == 0 {
		return nil, fmt.Errorf("opencode: no content in response")
	}

	return &ChatResponse{
		Content:      content.String(),
		FinishReason: "stop",
		Usage:        usage,
	}, nil
}
