package cliinstall

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/skills"
)

// Installer manages CLI binary downloads and installation from GitHub Releases.
// Creates its own GitHubClient from env vars — independent of the shared default installer.
type Installer struct {
	client      *skills.GitHubClient
	binDir      string
	allowAll    bool
	allowedOrgs []string
	mu          sync.Mutex // serializes bin dir writes + manifest mutations
}

// NewInstaller creates an installer for CLI binaries.
// token is the GitHub PAT (from GOCLAW_PACKAGES_GITHUB_TOKEN).
// binDir is where binaries are installed. AllowedOrgs restricts which orgs to download from.
func NewInstaller(token, binDir string, allowedOrgs []string) *Installer {
	if binDir == "" {
		binDir = defaultBinDir()
	}
	inst := &Installer{
		client:      skills.NewGitHubClient(token),
		binDir:      binDir,
		allowedOrgs: allowedOrgs,
		allowAll:    len(allowedOrgs) == 0,
	}
	return inst
}

// BinDir returns the binary install directory.
func (in *Installer) BinDir() string { return in.binDir }

// defaultBinDir returns the OS-specific default binary directory.
func defaultBinDir() string {
	dataDir := config.ResolvedDataDirFromEnv()
	return filepath.Join(dataDir, ".runtime", "bin")
}

// isAllowed checks if an org is in the allowlist.
func (in *Installer) isAllowed(owner string) bool {
	if in.allowAll {
		return true
	}
	owner = strings.ToLower(owner)
	return slices.Contains(in.allowedOrgs, owner)
}

// Install downloads and installs the latest release for a CLISpec.
func (in *Installer) Install(ctx context.Context, spec CLISpec) (*InstallResult, error) {
	if !in.isAllowed(spec.Owner) {
		return nil, fmt.Errorf("cliinstall: org %q not in allowlist", spec.Owner)
	}

	release, err := in.client.GetRelease(ctx, spec.Owner, spec.Repo, "")
	if err != nil {
		return nil, fmt.Errorf("cliinstall: fetch release for %s/%s: %w", spec.Owner, spec.Repo, err)
	}

	asset, err := skills.SelectAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, fmt.Errorf("cliinstall: select asset for %s: %w", spec.Binary, err)
	}

	maxBytes := int64(200 * 1024 * 1024) // 200 MB cap
	tmpPath, sha, err := in.client.DownloadAsset(ctx, asset.DownloadURL, maxBytes)
	if err != nil {
		return nil, fmt.Errorf("cliinstall: download %s: %w", spec.Binary, err)
	}
	defer os.Remove(tmpPath)

	if ca := skills.FindChecksumAsset(release, asset.Name); ca != nil {
		csPath, _, cerr := in.client.DownloadAsset(ctx, ca.DownloadURL, 1<<20)
		if cerr == nil {
			defer os.Remove(csPath)
			data, rerr := os.ReadFile(csPath)
			if rerr == nil {
				if sums, perr := skills.ParseChecksums(data); perr == nil {
					if expected, ok := sums[asset.Name]; ok {
						if verr := skills.VerifyChecksum(expected, sha); verr != nil {
							return nil, verr
						}
					}
				}
			}
		}
	}

	files, err := skills.ExtractArchiveAs(tmpPath, spec.Repo, 2*maxBytes)
	if err != nil {
		return nil, fmt.Errorf("cliinstall: extract %s: %w", spec.Binary, err)
	}

	binaries := skills.PickBinaries(files, spec.Repo)
	if len(binaries) == 0 {
		return nil, fmt.Errorf("cliinstall: no binary found in %s release", spec.Binary)
	}

	for idx := range binaries {
		if err := validateBinary(binaries[idx].Content); err != nil {
			return nil, fmt.Errorf("cliinstall: validate %s: %w", spec.Binary, err)
		}
	}

	// Commit to disk under lock.
	in.mu.Lock()
	defer in.mu.Unlock()

	if err := os.MkdirAll(in.binDir, 0o755); err != nil {
		return nil, fmt.Errorf("cliinstall: create bin dir: %w", err)
	}

	// Load manifest to detect version changes.
	m, _ := in.loadManifest()

	var writtenPaths []string
	for _, b := range binaries {
		name := filepath.Base(b.Name)
		dst := filepath.Join(in.binDir, name)
		if err := os.WriteFile(dst, b.Content, 0o755); err != nil {
			return nil, fmt.Errorf("cliinstall: write %s: %w", name, err)
		}
		writtenPaths = append(writtenPaths, name)
	}

	// Determine binary path — prefer the binary matching the spec name.
	binPath := filepath.Join(in.binDir, spec.Binary)
	if _, err := os.Stat(binPath); err != nil {
		if len(writtenPaths) > 0 {
			binPath = filepath.Join(in.binDir, writtenPaths[0])
		}
	}

	prevTag := ""
	newInstall := true
	for i, e := range m.Entries {
		if e.Name == spec.Binary {
			prevTag = e.Tag
			newInstall = false
			m.Entries[i].Tag = release.TagName
			m.Entries[i].UpdatedAt = time.Now().UTC()
			m.Entries[i].Binary = filepath.Base(binPath)
			goto save
		}
	}

	m.Entries = append(m.Entries, CLIManifestEntry{
		Name:         spec.Binary,
		ProviderType: spec.ProviderType,
		Repo:         spec.Owner + "/" + spec.Repo,
		Tag:          release.TagName,
		Binary:       filepath.Base(binPath),
		InstalledAt:  time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	})

save:
	if err := in.saveManifest(m); err != nil {
		slog.Warn("cliinstall: failed to save manifest", "error", err)
	}

	return &InstallResult{
		Spec:        spec,
		Version:     release.TagName,
		Path:        binPath,
		NewInstall:  newInstall,
		PreviousTag: prevTag,
	}, nil
}
