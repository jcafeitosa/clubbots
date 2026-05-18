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

// CopilotProvider shells out to `copilot -p` for non-interactive chat.
type CopilotProvider struct {
	name         string
	cliPath      string
	defaultModel string
	baseWorkDir  string
	mu           sync.Mutex
	sessionMu    sync.Map
}

type CopilotOption func(*CopilotProvider)

func WithCopilotName(name string) CopilotOption {
	return func(p *CopilotProvider) {
		if name != "" {
			p.name = name
		}
	}
}

func WithCopilotModel(model string) CopilotOption {
	return func(p *CopilotProvider) {
		if model != "" {
			p.defaultModel = model
		}
	}
}

func NewCopilotProvider(cliPath string, opts ...CopilotOption) *CopilotProvider {
	if cliPath == "" {
		cliPath = "copilot"
	}
	p := &CopilotProvider{
		name:         "copilot",
		cliPath:      cliPath,
		defaultModel: "gpt-5.4",
		baseWorkDir:  filepath.Join(config.ResolvedDataDirFromEnv(), "cli-workspaces", "copilot"),
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
	args := []string{"-p", userMsg, "--output-format", "json", "--model", model, "--allow-all-tools"}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Dir = workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdin = nil

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("copilot: %w (stderr: %s)", err, stderr.String())
	}

	return parseCopilotJSONL(output)
}

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
	args := []string{"-p", userMsg, "--output-format", "json", "--model", model, "--allow-all-tools"}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = workDir

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("copilot stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("copilot start: %w", err)
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

		var ev copilotJSONLEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "assistant.message_delta":
			if ev.Data != nil && ev.Data.DeltaContent != "" {
				contentBuf.WriteString(ev.Data.DeltaContent)
				onChunk(StreamChunk{Content: ev.Data.DeltaContent})
			}
		case "assistant.message":
			if ev.Data != nil && ev.Data.Content != "" {
				finalResp.Content = ev.Data.Content
			}
		case "result":
			if finalResp.Content == "" {
				finalResp.Content = contentBuf.String()
			}
			finalResp.FinishReason = "stop"
			if ev.ExitCode != nil && *ev.ExitCode != 0 {
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
		return nil, fmt.Errorf("copilot: %w (stderr: %s)", err, stderrBuf.String())
	}

	if finalResp.Content == "" {
		finalResp.Content = contentBuf.String()
		finalResp.FinishReason = "stop"
	}

	onChunk(StreamChunk{Done: true})
	return &finalResp, nil
}

func (p *CopilotProvider) ensureWorkDir(sessionKey string) string {
	safe := sanitizePathSegment(sessionKey)
	dir := filepath.Join(p.baseWorkDir, safe)
	p.mu.Lock()
	defer p.mu.Unlock()
	os.MkdirAll(dir, 0755)
	return dir
}

func (p *CopilotProvider) lockSession(sessionKey string) func() {
	actual, _ := p.sessionMu.LoadOrStore(sessionKey, &sync.Mutex{})
	m := actual.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// copilotJSONLEvent represents a single JSONL line from `copilot --output-format json`.
type copilotJSONLEvent struct {
	Type     string         `json:"type"`
	Data     *copilotData   `json:"data,omitempty"`
	ExitCode *int           `json:"exitCode,omitempty"`
}

type copilotData struct {
	Content      string `json:"content"`
	DeltaContent string `json:"deltaContent"`
}

func parseCopilotJSONL(data []byte) (*ChatResponse, error) {
	lines := bytes.Split(data, []byte("\n"))
	var content string

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var ev copilotJSONLEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type == "assistant.message" && ev.Data != nil {
			content = ev.Data.Content
		}
	}

	if content == "" {
		// Fallback: try message_delta events
		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			var ev copilotJSONLEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				continue
			}
			if ev.Type == "assistant.message_delta" && ev.Data != nil {
				content += ev.Data.DeltaContent
			}
		}
	}

	if content == "" {
		return nil, fmt.Errorf("copilot: no content in response")
	}

	return &ChatResponse{
		Content:      content,
		FinishReason: "stop",
	}, nil
}
