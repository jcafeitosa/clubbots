// Package cliinstall provides CLI binary detection, installation, and update management
// for subprocess-based LLM providers. It is independent of the providers package.
package cliinstall

import "time"

// CLISpec describes a known CLI-based LLM tool.
type CLISpec struct {
	ProviderType string // "claude_cli", "codex_cli", "copilot", "opencode"
	Binary       string // "claude", "codex", "copilot", "opencode"
	Owner        string // GitHub org: "anthropics", "openai", "github", "opencode-ai"
	Repo         string // GitHub repo: "claude-code", "codex", "copilot-cli", "opencode"
	DefaultModel string // "sonnet", "gpt-5.4", etc.
}

// KnownCLIs is the authoritative catalog of all supported CLI tools.
var KnownCLIs = []CLISpec{
	{ProviderType: "claude_cli", Binary: "claude", Owner: "anthropics", Repo: "claude-code", DefaultModel: "sonnet"},
	{ProviderType: "codex_cli", Binary: "codex", Owner: "openai", Repo: "codex", DefaultModel: "gpt-5.4"},
	{ProviderType: "copilot", Binary: "copilot", Owner: "github", Repo: "copilot-cli", DefaultModel: "gpt-5.4"},
	{ProviderType: "opencode", Binary: "opencode", Owner: "opencode-ai", Repo: "opencode", DefaultModel: "gpt-5.4"},
}

// InstallResult summarizes an install or update operation.
type InstallResult struct {
	Spec        CLISpec
	Version     string // installed tag (e.g. "v1.0.37")
	Path        string // installed binary path
	NewInstall  bool   // true = fresh install, false = update
	PreviousTag string // previous version (empty for new install)
}

// CLIManifestEntry records an installed CLI.
type CLIManifestEntry struct {
	Name         string    `json:"name"`          // "claude", "codex", etc.
	ProviderType string    `json:"provider_type"` // "claude_cli", etc.
	Repo         string    `json:"repo"`          // "anthropics/claude-code"
	Tag          string    `json:"tag"`           // "v1.0.37"
	Binary       string    `json:"binary"`        // "claude"
	InstalledAt  time.Time `json:"installed_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CLIManifest is the persisted state for installed CLI binaries.
type CLIManifest struct {
	Version int                `json:"version"` // 1
	Entries []CLIManifestEntry `json:"entries"`
}
