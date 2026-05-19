package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// STTProvider converts speech audio to text.
type STTProvider interface {
	Transcribe(ctx context.Context, audioData []byte, format string) (string, error)
	Name() string
}

// STTManager manages speech-to-text providers.
type STTManager struct {
	providers map[string]STTProvider
	default_  string
}

// NewSTTManager creates an STT manager with built-in providers.
func NewSTTManager() *STTManager {
	m := &STTManager{providers: make(map[string]STTProvider)}
	m.Register(NewWhisperLocalProvider())
	m.Register(NewOpenAISTTProvider())
	m.SetDefault("whisper-local")
	return m
}

// Register adds a provider.
func (m *STTManager) Register(p STTProvider) { m.providers[p.Name()] = p }

// SetDefault sets the default provider name.
func (m *STTManager) SetDefault(name string) { m.default_ = name }

// Transcribe runs speech-to-text on audio data.
func (m *STTManager) Transcribe(ctx context.Context, audioData []byte, format string) (string, error) {
	p, ok := m.providers[m.default_]
	if !ok {
		return "", fmt.Errorf("stt: no provider configured")
	}
	return p.Transcribe(ctx, audioData, format)
}

// WhisperLocalProvider uses a local whisper.cpp binary for STT.
type WhisperLocalProvider struct {
	modelPath string
}

func NewWhisperLocalProvider() *WhisperLocalProvider {
	return &WhisperLocalProvider{
		modelPath: os.Getenv("WHISPER_MODEL_PATH"),
	}
}

func (p *WhisperLocalProvider) Name() string { return "whisper-local" }

func (p *WhisperLocalProvider) Transcribe(ctx context.Context, audioData []byte, format string) (string, error) {
	if _, err := exec.LookPath("whisper-cpp"); err != nil {
		return "", fmt.Errorf("whisper-local: whisper-cpp not installed (brew install whisper-cpp)")
	}

	// Write audio to temp file
	tmpFile, err := os.CreateTemp("", "goclaw-stt-*.wav")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(audioData); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	args := []string{"-f", tmpFile.Name(), "-otxt"}
	if p.modelPath != "" {
		args = append(args, "-m", p.modelPath)
	}

	cmd := exec.CommandContext(ctx, "whisper-cpp", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("whisper-local: %w", err)
	}
	return string(bytes.TrimSpace(out)), nil
}

// OpenAISTTProvider uses OpenAI's transcription API.
type OpenAISTTProvider struct {
	apiKey string
	client *http.Client
}

func NewOpenAISTTProvider() *OpenAISTTProvider {
	return &OpenAISTTProvider{
		apiKey: os.Getenv("OPENAI_API_KEY"),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OpenAISTTProvider) Name() string { return "openai" }

func (p *OpenAISTTProvider) Transcribe(ctx context.Context, audioData []byte, format string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("openai stt: OPENAI_API_KEY not set")
	}

	body := bytes.NewReader(audioData)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "audio/"+format)

	// Use multipart form
	req.Header.Set("Content-Type", "multipart/form-data")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai stt: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Text, nil
}
