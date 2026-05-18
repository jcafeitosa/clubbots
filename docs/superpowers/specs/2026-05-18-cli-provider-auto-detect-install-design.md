# CLI Provider Auto-Detection & Installation

## Summary

Add automatic detection, installation, and periodic update of CLI-based LLM providers
(Claude CLI, Codex CLI, GitHub Copilot, OpenCode). When a CLI binary is explicitly
configured but missing, the system downloads it from GitHub Releases. When a supported
CLI is found on PATH but not configured, the system reports availability. Periodic
checks keep installed CLIs up-to-date.

## Motivation

Today only Claude CLI supports subprocess-based invocation, and even that requires
manual `cli_path` configuration. If the binary is not found via `exec.LookPath`, the
provider is silently skipped. There is no mechanism to install missing CLIs or detect
new ones.

This feature eliminates manual setup for CLI providers, making the system
self-bootstrapping while respecting operator control (auto-install only for explicitly
configured providers).

## CLI Catalog

| Provider Type   | Binary Name | GitHub Repo              | Default Model     |
|-----------------|-------------|--------------------------|-------------------|
| `claude_cli`    | `claude`    | `anthropics/claude-code` | `sonnet`          |
| `codex_cli`     | `codex`     | `openai/codex`           | `gpt-5.4`         |
| `copilot`       | `copilot`   | `github/copilot-cli`     | `gpt-5.4`         |
| `opencode`      | `opencode`  | `opencode-ai/opencode`   | `gpt-5.4`         |

`claude_cli` already exists in `store/provider_store.go`. The other three are new.

## Architecture

### New Package: `internal/cliinstall/`

```
internal/cliinstall/
├── registry.go         # CLI catalog (KnownCLIs, metadata)
├── detect.go           # Cross-platform binary detection
├── detect_darwin.go    # macOS-specific paths (Homebrew, /usr/local/bin, etc.)
├── detect_linux.go     # Linux-specific paths
├── install.go          # Download + install orchestration
├── install_darwin.go   # Mach-O validation + asset selection for macOS
├── install_linux.go    # ELF validation + asset selection for Linux
├── update.go           # Periodic update check logic
├── update_test.go      # Update tests
├── manifest.go         # Local install manifest (version tracking)
└── manifest_test.go    # Manifest tests
```

This package has **zero dependency on providers** — it is pure CLI lifecycle
infrastructure, reusable by any provider or external caller.

### Key Types

```go
// CLISpec describes a known CLI tool.
type CLISpec struct {
    ProviderType string // "claude_cli", "codex_cli", "copilot", "opencode"
    Binary       string // "claude", "codex", "copilot", "opencode"
    Owner        string // "anthropics", "openai", "github", "opencode-ai"
    Repo         string // "claude-code", "codex", "copilot-cli", "opencode"
    DefaultModel string // default model alias
}

// InstallResult summarizes an install/update operation.
type InstallResult struct {
    Spec        CLISpec
    Version     string // installed tag
    Path        string // installed binary path
    NewInstall  bool   // true = fresh install, false = update
    PreviousTag string // previous version (empty if new install)
}
```

### Existing Code Reuse

| Existing Component | How Reused |
|--------------------|------------|
| `skills/github_installer.go` | Extract GitHub API client, asset selection heuristics, checksum verification into shared helpers. The existing `GitHubInstaller` continues to serve skill dependencies (Linux-only). We add cross-platform support via new functions in `cliinstall/`. |
| `skills/github_client.go` | Reuse `GitHubClient` type for GitHub API calls. |
| `skills/github_checksum.go` | Reuse `FindChecksumAsset`, `ParseChecksums`, `VerifyChecksum` for binary verification. |
| `skills/archive_extract.go` | Reuse `ExtractArchiveAs` for tar.gz/zip extraction. |
| `providers/claude_cli.go` | `ClaudeCLIProvider` is the reference subprocess pattern: spawn, session locking, stdin/stdout communication, error classification. New CLI providers follow this same architecture. |
| `providers/acp/process.go` | Reuse process spawning with context, lifecycle management, idle reaping patterns (extract shared utilities). |
| `cmd/gateway_providers.go` | Extend `registerProviders` and `registerProvidersFromDB` with CLI auto-detect/install hooks via new `registerCLIProviders()` function. |

## Behavior: Detection & Decision Flow

```
STARTUP
  │
  ├── 1. Load providers from config.json + DB (existing flow, unchanged)
  │
  ├── 2. For each CLI in cliinstall.KnownCLIs:
  │     │
  │     ├── ALREADY REGISTERED (in config or DB)
  │     │     ├── exec.LookPath(binary) → found?
  │     │     │   ├── YES → register provider normally
  │     │     │   └── NO  → AUTO-INSTALL (download GitHub latest)
  │     │     │              ├── success → register provider
  │     │     │              └── failure → log ERROR, skip provider
  │     │
  │     └── NOT REGISTERED
  │           ├── exec.LookPath(binary) → found?
  │           │   ├── YES → log INFO "cli available but not configured: <name>"
  │           │   └── NO  → silent (no noise for unrelated CLIs)
  │           └── Do NOT auto-register
  │
  └── 3. Update check (every startup + weekly via scheduler lane)
```

### Rationale

- **Registered = operator intent.** If someone added a `claude_cli` provider in the DB,
  they want it working. Auto-install is justified.
- **Detected-only = not configured.** Inform without acting. Prevents surprise provider
  activation and respects the principle of least privilege.
- **Silent when neither detected nor configured.** No log spam for CLIs the operator
  doesn't care about.

## Cross-Platform Installation

### Asset Selection

Asset selection adapts the existing `SelectAsset` logic from `skills/github_installer.go`
but adds macOS support:

| Platform | OS Filter      | Arch Filter              | Binary Validation |
|----------|---------------|--------------------------|-------------------|
| Linux    | `(?i)linux`    | amd64 or arm64           | ELF magic + class |
| macOS    | `(?i)(darwin\|macos\|mac)` | amd64 or arm64 | Mach-O magic + arch |

### Mach-O Validation (new)

```go
// validateMachO checks magic bytes, 64-bit class, and arch matches runtime.
func validateMachO(content []byte) error {
    if len(content) < 4 {
        return ErrNotMachO
    }
    // Magic: 0xFEEDFACF (64-bit) or 0xFEEDFACE (32-bit, rejected)
    magic := binary.BigEndian.Uint32(content[:4])
    if magic != 0xFEEDFACF {
        if magic == 0xFEEDFACE {
            return ErrUnsupportedMachOClass
        }
        // Also check fat binary magic: 0xCAFEBABE, 0xBEBAFECA
        return ErrNotMachO
    }
    // Check CPU type at offset 4
    cpuType := binary.LittleEndian.Uint32(content[4:8])
    // CPU_TYPE_ARM64 = 0x0100000C, CPU_TYPE_X86_64 = 0x01000007
    wantCPU := uint32(0x01000007) // x86_64
    if runtime.GOARCH == "arm64" {
        wantCPU = 0x0100000C
    }
    if cpuType != wantCPU {
        return fmt.Errorf("%w (binary=0x%X, runtime=%s)", ErrMachOArchMismatch, cpuType, runtime.GOARCH)
    }
    return nil
}
```

### Install Directory

- **Linux:** `~/.goclaw/data/.runtime/bin/` (existing convention)
- **macOS:** `~/Library/Application Support/GoClaw/runtime/bin/`
- Configurable via `GOCLAW_CLI_BIN_DIR` env var (overrides default)

### Manifest

Local JSON file tracking installed CLI versions at
`<data-dir>/.runtime/cli-manifest.json`:

```json
{
  "version": 1,
  "entries": [
    {
      "name": "claude",
      "provider_type": "claude_cli",
      "repo": "anthropics/claude-code",
      "tag": "v1.0.37",
      "binary": "claude",
      "installed_at": "2026-05-18T14:30:00Z",
      "updated_at": "2026-05-18T14:30:00Z"
    }
  ]
}
```

## Periodic Update Check

### Trigger

- On gateway startup (after provider registration)
- Weekly via existing scheduler lane (cron: `0 7 * * 3` — Wednesday 7am local)
- Disable via `GOCLAW_CLI_AUTO_UPDATE=false`

### Flow

```
UpdateCheck(spec CLISpec)
  │
  ├── GET /repos/:owner/:repo/releases/latest (GitHub API)
  │     └── Cache tag for 1h to avoid rate limiting
  │
  ├── Compare tag with manifest entry
  │     ├── Same → no-op
  │     └── Different → download & validate new binary
  │
  └── Atomic swap with rollback:
        ├── Write new binary as "<name>.new"
        ├── Validate (ELF/Mach-O)
        ├── chmod <name>.new 0755
        ├── Rename current → "<name>.previous"
        ├── Rename "<name>.new" → "<name>"
        ├── Remove "<name>.previous"
        └── Update manifest
```

### Atomic Swap Safety

The swap is crash-safe: if the process dies at any point, at least one valid binary
remains. On next startup, if `<name>.new` exists without a corresponding `<name>`,
the system promotes it. If `<name>.previous` exists, it is cleaned up.

### GitHub API Rate Limiting

- Without token: 60 req/h → with 4 CLIs, startup check uses 4 requests; cached for 1h
- With `GOCLAW_PACKAGES_GITHUB_TOKEN`: 5000 req/h → no practical limit
- Update check respects `X-RateLimit-Remaining` header; backs off if near limit

## Provider Integration

### New Provider Types

In `internal/store/provider_store.go`:

```go
ProviderCodexCLI  = "codex_cli"  // OpenAI Codex CLI (subprocess)
ProviderCopilot   = "copilot"     // GitHub Copilot CLI (subprocess)
ProviderOpenCode  = "opencode"    // OpenCode CLI (subprocess)
```

Added to `ValidProviderTypes` map. No DB migration needed — `provider_type` is
`VARCHAR(30)` without a CHECK constraint; validation is in Go code only.

### Registration in gateway_providers.go

```go
func registerCLIProviders(registry *providers.Registry, cfg *config.Config,
    installer *cliinstall.Installer, provStore store.ProviderStore) {

    for _, spec := range cliinstall.KnownCLIs {
        configured := isConfigured(cfg, provStore, spec)
        found := cliinstall.Detect(spec.Binary)

        switch {
        case configured && found:
            registerCLIProvider(registry, spec, foundPath)
        case configured && !found:
            result, err := installer.Install(context.Background(), spec)
            if err != nil {
                slog.Error("cliinstall: failed to install", "binary", spec.Binary, "error", err)
                continue
            }
            registerCLIProvider(registry, spec, result.Path)
            slog.Info("cliinstall: installed and registered", "binary", spec.Binary, "version", result.Version)
        case !configured && found:
            slog.Info("cli available but not configured", "binary", spec.Binary,
                "provider_type", spec.ProviderType,
                "hint", "add a provider in config or DB to enable")
        case !configured && !found:
            // silent
        }
    }
}
```

### CLI Provider Registration

Each CLI uses a subprocess-based provider. Claude CLI already has `ClaudeCLIProvider`
(in `internal/providers/claude_cli.go`). For Codex CLI, Copilot, and OpenCode, we
follow the same pattern — thin subprocess wrappers that shell out to the binary:

```go
func registerCLIProvider(registry *providers.Registry, spec cliinstall.CLISpec, binaryPath string) {
    switch spec.ProviderType {
    case "claude_cli":
        // Existing ClaudeCLIProvider pattern
        registry.Register(providers.NewClaudeCLIProvider(binaryPath,
            providers.WithClaudeCLIModel(spec.DefaultModel),
            providers.WithClaudeCLISecurityHooks("", true),
        ))
    case "codex_cli":
        registry.Register(providers.NewCodexCLIProvider(binaryPath,
            providers.WithCodexCLIModel(spec.DefaultModel)))
    case "copilot":
        registry.Register(providers.NewCopilotProvider(binaryPath))
    case "opencode":
        registry.Register(providers.NewOpenCodeProvider(binaryPath))
    }
    slog.Info("registered CLI provider", "type", spec.ProviderType, "binary", binaryPath)
}
```

### Why Subprocess Providers (Not ACP)

The ACP (Agent Client Protocol) is Anthropic's specific wire protocol for agent
binaries. Codex CLI, GitHub Copilot, and OpenCode each use their own different CLI
interface — they are not ACP-compatible.

However, they all share the same subprocess execution pattern already proven by
`ClaudeCLIProvider`: spawn binary → write prompt to stdin → stream/read response from
stdout → manage per-session concurrency with mutexes. The new providers reuse
patterns from `ClaudeCLIProvider` (session locking, error classification, timeout
handling) without requiring the ACP wire protocol.

The shared process-level infrastructure from `internal/providers/acp/process.go`
(spawn with context, lifecycle management, idle reaping) can be extracted and reused
by the new CLI providers in a follow-up refactor.

## Configuration

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `GOCLAW_CLI_BIN_DIR` | Override binary install directory | OS-specific |
| `GOCLAW_CLI_AUTO_UPDATE` | Enable/disable periodic updates | `true` |
| `GOCLAW_PACKAGES_GITHUB_TOKEN` | GitHub PAT (reused) | (none) |
| `GOCLAW_PACKAGES_GITHUB_ALLOWED_ORGS` | Org allowlist (reused) | (all allowed) |

### Config File (config.json)

CLI providers can be explicitly configured like any other provider:

```json5
{
  "providers": {
    "claude_cli": {
      "cli_path": "/usr/local/bin/claude",
      "model": "sonnet"
    }
  }
}
```

The `cli_path` field (existing) triggers the auto-install flow when the binary is
missing. If `cli_path` is absent but the provider is registered in DB, the system
falls back to `exec.LookPath("claude")` and then auto-install.

## Security

- **Org allowlist:** Only downloads from orgs in `GOCLAW_PACKAGES_GITHUB_ALLOWED_ORGS`
  (if set). Default: `anthropics`, `openai`, `github`, `opencode-ai` are pre-allowed.
- **Binary validation:** ELF magic (Linux) or Mach-O magic (macOS) mandatory before
  execution. Architecture must match runtime.
- **Checksum verification:** If the GitHub release includes checksum assets, verify
  before writing binary.
- **No script execution:** Downloads raw binary assets only. Never pipes to shell.
- **Path safety:** Binaries installed to fixed directory under data dir. No path
  traversal possible.
- **Permission bits:** Binary written with 0755, never 0777 or SUID.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| GitHub API unreachable | Log warning, skip install. Retry next startup. |
| No matching asset for OS/arch | Log error with available asset names. Skip. |
| Binary fails ELF/Mach-O validation | Delete downloaded file. Log error. Skip. |
| Checksum mismatch | Delete downloaded file. Log ERROR (security concern). Skip. |
| Disk full during write | Log error. Clean up partial file. Skip. |
| Rate limited by GitHub | Back off, log at debug level. Try cached tag. |
| Update fails (any reason) | Keep existing binary. Log warning. Retry next cycle. |

All install/update failures are **non-fatal to gateway startup** — the system
continues without the affected provider.

## Migration

- New provider types added to DB enums (both PG migration + SQLite schema update per
  `CLAUDE.md` conventions).
- Existing Claude CLI providers in DB continue to work unchanged.
- No breaking changes to config format or existing provider registration.

## Tasks

| Phase | Description | Dependencies |
|-------|-------------|--------------|
| F1 | `internal/cliinstall/registry.go` + `detect*.go` — CLI catalog + cross-platform detection | none |
| F2 | `internal/cliinstall/install.go` — download orchestration (cross-platform) | F1 |
| F3 | `internal/cliinstall/install_darwin.go` — Mach-O validation + macOS asset selection | F2 |
| F4 | `internal/cliinstall/install_linux.go` — ELF validation + Linux asset selection (reuse existing) | F2 |
| F5 | `internal/cliinstall/manifest.go` — local manifest read/write | F1 |
| F6 | `internal/cliinstall/update.go` — periodic update check + atomic swap | F2, F5 |
| F7 | New provider types in `store/provider_store.go` + migration (PG + SQLite) | F1 |
| F8 | Extend `cmd/gateway_providers.go` — `registerCLIProviders()` | F6, F7 |
| F9 | Integration tests (detect, install, update, registration) | F3-F8 |
| F10 | Disable `GOCLAW_CLI_AUTO_UPDATE` knob + env var wiring | F6 |

## Open Questions

1. **Codex CLI vs Codex HTTP:** Today `codex` means the HTTP-based ChatGPT OAuth
   provider. Should the CLI variant use `codex_cli` (as proposed) or a different
   identifier? **Decision: `codex_cli` to avoid ambiguity.**

2. **Homebrew integration on macOS:** Should we also check Homebrew install paths
   (`/opt/homebrew/bin`, `/usr/local/bin`)? **Decision: yes, in detect_darwin.go.**

3. **Windows support:** Not in scope for this iteration. The `-tags sqliteonly` build
   also supports Windows (desktop app). Windows binary validation (PE/COFF) would be
   a follow-up.
