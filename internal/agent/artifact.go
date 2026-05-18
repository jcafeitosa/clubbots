package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// Artifact represents a persistent, shareable output document.
// Inspired by Claude's artifact system — permanent documents vs inline visuals.
type Artifact struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"` // "markdown", "html", "svg", "json", "text"
	SessionKey  string    `json:"session_key"`
	AgentID     string    `json:"agent_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// ArtifactStore persists and queries artifacts.
type ArtifactStore struct {
	dir string // storage directory
}

// NewArtifactStore creates an artifact store under the given data directory.
func NewArtifactStore(dataDir string) *ArtifactStore {
	dir := filepath.Join(dataDir, "artifacts")
	os.MkdirAll(dir, 0755)
	return &ArtifactStore{dir: dir}
}

// NewArtifactStoreFromConfig creates an artifact store from the resolved data dir.
func NewArtifactStoreFromConfig() *ArtifactStore {
	return NewArtifactStore(config.ResolvedDataDirFromEnv())
}

// Save persists an artifact to disk as JSON.
func (s *ArtifactStore) Save(ctx context.Context, a *Artifact) error {
	if a.ID == "" {
		a.ID = generateArtifactID(a.Title, a.Content)
	}
	now := time.Now()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("artifact marshal: %w", err)
	}

	path := filepath.Join(s.dir, a.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// Load reads an artifact by ID.
func (s *ArtifactStore) Load(ctx context.Context, id string) (*Artifact, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("artifact load: %w", err)
	}
	var a Artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("artifact unmarshal: %w", err)
	}
	return &a, nil
}

// List returns all artifacts, optionally filtered by session.
func (s *ArtifactStore) List(ctx context.Context, sessionKey string) ([]*Artifact, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var artifacts []*Artifact
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		a, err := s.Load(ctx, e.Name()[:len(e.Name())-5])
		if err != nil {
			continue
		}
		if sessionKey == "" || a.SessionKey == sessionKey {
			artifacts = append(artifacts, a)
		}
	}
	return artifacts, nil
}

// Delete removes an artifact by ID.
func (s *ArtifactStore) Delete(ctx context.Context, id string) error {
	path := filepath.Join(s.dir, id+".json")
	return os.Remove(path)
}

// Export writes the artifact content to a file in the specified format.
func (s *ArtifactStore) Export(ctx context.Context, id, outputPath string) error {
	a, err := s.Load(ctx, id)
	if err != nil {
		return err
	}
	ext := ".md"
	switch a.ContentType {
	case "html":
		ext = ".html"
	case "svg":
		ext = ".svg"
	case "json":
		ext = ".json"
	}
	if filepath.Ext(outputPath) == "" {
		outputPath += ext
	}
	return os.WriteFile(outputPath, []byte(a.Content), 0644)
}

func generateArtifactID(title, content string) string {
	h := sha256.Sum256([]byte(title + content))
	return fmt.Sprintf("art_%x", h[:8])
}
