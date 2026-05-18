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

// CodexCLIProvider shells out to `codex exec` for non-interactive chat.
type CodexCLIProvider struct {
	name         string
	cliPath      string
	defaultModel string
	baseWorkDir  string
	mu           sync.Mutex
	sessionMu    sync.Map
}

type CodexCLIOption func(*CodexCLIProvider)

func WithCodexCLIName(name string) CodexCLIOption {
	return func(p *CodexCLIProvider) {
		if name != "" {
			p.name = name
		}
	}
}

func WithCodexCLIModel(model string) CodexCLIOption {
	return func(p *CodexCLIProvider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

func WithCodexCLIWorkDir(dir string) CodexCLIOption {
	return func(p *CodexCLIProvider) {
		if dir != "" {
			p.baseWorkDir = dir
		}
	}
}

func NewCodexCLIProvider(cliPath string, opts ...CodexCLIOption) *CodexCLIProvider {
	if cliPath == "" {
		cliPath = "codex"
	}
	p := &CodexCLIProvider{
		name:         "codex-cli",
		cliPath:      cliPath,
		defaultModel: "gpt-5.4",
		baseWorkDir:  filepath.Join(config.ResolvedDataDirFromEnv(), "cli-workspaces", "codex"),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *CodexCLIProvider) Name() string        { return p.name }
func (p *CodexCLIProvider) DefaultModel() string { return p.defaultModel }

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

func (p *CodexCLIProvider) Close() error { return nil }

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
	args := []string{"exec", "--json", "-m", model, "--ephemeral", "--dangerously-bypass-approvals-and-sandbox", userMsg}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = nil

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("codex-cli: %w (stderr: %s)", err, stderr.String())
	}

	return parseCodexJSONL(output)
}

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
	args := []string{"exec", "--json", "-m", model, "--ephemeral", "--dangerously-bypass-approvals-and-sandbox", userMsg}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = workDir

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil // prevent reading from parent stdin

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

		var ev codexCLIJSONLEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "item.completed":
			if ev.Item != nil && ev.Item.Type == "agent_message" && ev.Item.Text != "" {
				contentBuf.WriteString(ev.Item.Text)
				onChunk(StreamChunk{Content: ev.Item.Text})
			}
		case "turn.completed":
			finalResp.Content = contentBuf.String()
			finalResp.FinishReason = "stop"
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

func (p *CodexCLIProvider) ensureWorkDir(sessionKey string) string {
	safe := sanitizePathSegment(sessionKey)
	dir := filepath.Join(p.baseWorkDir, safe)
	p.mu.Lock()
	defer p.mu.Unlock()
	os.MkdirAll(dir, 0755)
	return dir
}

func (p *CodexCLIProvider) lockSession(sessionKey string) func() {
	actual, _ := p.sessionMu.LoadOrStore(sessionKey, &sync.Mutex{})
	m := actual.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// codexCLIJSONLEvent represents a single JSONL line from `codex exec --json`.
type codexCLIJSONLEvent struct {
	Type  string       `json:"type"`
	Item  *codexCLIItem   `json:"item,omitempty"`
	Usage *codexCLIUsage  `json:"usage,omitempty"`
}

type codexCLIItem struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "agent_message"
	Text string `json:"text"`
}

type codexCLIUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func parseCodexJSONL(data []byte) (*ChatResponse, error) {
	lines := bytes.Split(data, []byte("\n"))
	var content strings.Builder
	var usage *Usage

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev codexCLIJSONLEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type == "item.completed" && ev.Item != nil && ev.Item.Text != "" {
			content.WriteString(ev.Item.Text)
		}
		if ev.Usage != nil {
			usage = &Usage{
				PromptTokens:     ev.Usage.InputTokens,
				CompletionTokens: ev.Usage.OutputTokens,
				TotalTokens:      ev.Usage.InputTokens + ev.Usage.OutputTokens,
			}
		}
	}

	if content.Len() == 0 {
		return nil, fmt.Errorf("codex-cli: no content in response")
	}

	return &ChatResponse{
		Content:      content.String(),
		FinishReason: "stop",
		Usage:        usage,
	}, nil
}
