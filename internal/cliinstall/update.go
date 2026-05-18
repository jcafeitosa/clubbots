package cliinstall

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// CheckForUpdate compares the installed version with the latest GitHub release.
// Returns nil if up-to-date or if the check fails (non-fatal).
func (in *Installer) CheckForUpdate(ctx context.Context, spec CLISpec) (*InstallResult, error) {
	m, err := in.loadManifest()
	if err != nil {
		return nil, fmt.Errorf("cliinstall: load manifest: %w", err)
	}

	// Find existing entry.
	var entry *CLIManifestEntry
	for i := range m.Entries {
		if m.Entries[i].Name == spec.Binary {
			entry = &m.Entries[i]
			break
		}
	}
	if entry == nil {
		// Not installed — only install if explicitly requested (not during update check).
		return nil, nil
	}

	release, err := in.client.GetRelease(ctx, spec.Owner, spec.Repo, "")
	if err != nil {
		slog.Warn("cliinstall: update check failed", "binary", spec.Binary, "error", err)
		return nil, nil
	}

	if release.TagName == entry.Tag {
		return nil, nil // up-to-date
	}

	slog.Info("cliinstall: update available",
		"binary", spec.Binary,
		"current", entry.Tag,
		"latest", release.TagName,
	)

	// Reuse full install flow — Install() handles update (manifest already has entry).
	result, err := in.Install(ctx, spec)
	if err != nil {
		slog.Warn("cliinstall: update failed", "binary", spec.Binary, "error", err)
		return nil, nil
	}

	slog.Info("cliinstall: updated",
		"binary", spec.Binary,
		"version", result.Version,
		"previous", result.PreviousTag,
	)
	return result, nil
}

// CheckAllForUpdates runs CheckForUpdate for every known CLI.
// Errors are logged, not returned — individual failures don't block the batch.
func (in *Installer) CheckAllForUpdates(ctx context.Context) []*InstallResult {
	var results []*InstallResult
	for _, spec := range KnownCLIs {
		result, err := in.CheckForUpdate(ctx, spec)
		if err != nil {
			slog.Warn("cliinstall: update check error", "binary", spec.Binary, "error", err)
			continue
		}
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

// RecoverPartialUpdates cleans up orphan .new and .previous files from a crashed update.
func (in *Installer) RecoverPartialUpdates() error {
	entries, err := os.ReadDir(in.binDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".new" {
			fullPath := filepath.Join(in.binDir, name)
			baseName := name[:len(name)-4] // strip .new
			basePath := filepath.Join(in.binDir, baseName)
			if _, err := os.Stat(basePath); os.IsNotExist(err) {
				// .new exists but base doesn't — promote.
				if err := os.Rename(fullPath, basePath); err != nil {
					slog.Warn("cliinstall: failed to promote .new binary", "path", fullPath, "error", err)
				} else {
					slog.Info("cliinstall: promoted orphan .new binary", "path", basePath)
				}
			} else {
				// Both exist — remove .new (base is the canonical one).
				os.Remove(fullPath)
			}
		}
		if filepath.Ext(name) == ".previous" {
			// Stale rollback file — remove.
			os.Remove(filepath.Join(in.binDir, name))
		}
	}

	// Age out stale manifest entries (installed > 90 days ago with no binary on disk).
	if m, err := in.loadManifest(); err == nil {
		changed := false
		for i := len(m.Entries) - 1; i >= 0; i-- {
			if time.Since(m.Entries[i].InstalledAt) > 90*24*time.Hour {
				binPath := filepath.Join(in.binDir, m.Entries[i].Binary)
				if _, err := os.Stat(binPath); os.IsNotExist(err) {
					m.Entries = append(m.Entries[:i], m.Entries[i+1:]...)
					changed = true
				}
			}
		}
		if changed {
			in.saveManifest(m)
		}
	}

	return nil
}
