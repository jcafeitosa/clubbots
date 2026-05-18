package cliinstall

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// manifestPath returns the path to the CLI install manifest.
func (in *Installer) manifestPath() string {
	return filepath.Join(filepath.Dir(in.binDir), "cli-manifest.json")
}

// loadManifest returns an empty manifest if the file is missing.
func (in *Installer) loadManifest() (*CLIManifest, error) {
	path := in.manifestPath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CLIManifest{Version: 1}, nil
		}
		return nil, err
	}
	var m CLIManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("cliinstall: parse manifest: %w", err)
	}
	if m.Version == 0 {
		m.Version = 1
	}
	return &m, nil
}

// saveManifest writes atomically via temp file + fsync + rename.
func (in *Installer) saveManifest(m *CLIManifest) error {
	dir := filepath.Dir(in.manifestPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := in.manifestPath() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, in.manifestPath()); err != nil {
		os.Remove(tmp)
		return err
	}
	if d, derr := os.Open(dir); derr == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}
