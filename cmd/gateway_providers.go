package cmd

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/cliinstall"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/oauth"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
)

// loopbackAddr normalizes a gateway address for local connections.
// CLI processes on the same machine can't connect to 0.0.0.0 on some OSes.
func loopbackAddr(host string, port int) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func registerProviders(registry *providers.Registry, cfg *config.Config, modelReg providers.ModelRegistry, installer *cliinstall.Installer) {
	if cfg.Providers.Anthropic.APIKey != "" {
		registry.Register(providers.NewAnthropicProvider(cfg.Providers.Anthropic.APIKey,
			providers.WithAnthropicBaseURL(cfg.Providers.Anthropic.APIBase),
			providers.WithAnthropicRegistry(modelReg)))
		slog.Info("registered provider", "name", "anthropic")
	}

	if cfg.Providers.OpenAI.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("openai", cfg.Providers.OpenAI.APIKey, cfg.Providers.OpenAI.APIBase, "gpt-4o").
			WithRegistry(modelReg))
		slog.Info("registered provider", "name", "openai")
	}

	if cfg.Providers.OpenRouter.APIKey != "" {
		orProv := providers.NewOpenAIProvider("openrouter", cfg.Providers.OpenRouter.APIKey, "https://openrouter.ai/api/v1", "anthropic/claude-sonnet-4-5-20250929")
		orProv.WithSiteInfo("https://goclaw.sh", "GoClaw")
		registry.Register(orProv)
		slog.Info("registered provider", "name", "openrouter")
	}

	if cfg.Providers.Groq.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("groq", cfg.Providers.Groq.APIKey, "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile"))
		slog.Info("registered provider", "name", "groq")
	}

	if cfg.Providers.DeepSeek.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("deepseek", cfg.Providers.DeepSeek.APIKey, "https://api.deepseek.com/v1", "deepseek-chat"))
		slog.Info("registered provider", "name", "deepseek")
	}

	if cfg.Providers.Gemini.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("gemini", cfg.Providers.Gemini.APIKey, "https://generativelanguage.googleapis.com/v1beta/openai", "gemini-2.0-flash"))
		slog.Info("registered provider", "name", "gemini")
	}

	if cfg.Providers.Mistral.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("mistral", cfg.Providers.Mistral.APIKey, "https://api.mistral.ai/v1", "mistral-large-latest"))
		slog.Info("registered provider", "name", "mistral")
	}

	if cfg.Providers.XAI.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("xai", cfg.Providers.XAI.APIKey, "https://api.x.ai/v1", "grok-3-mini"))
		slog.Info("registered provider", "name", "xai")
	}

	if cfg.Providers.MiniMax.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("minimax", cfg.Providers.MiniMax.APIKey, "https://api.minimax.io/v1", "MiniMax-M2.5").
			WithChatPath("/text/chatcompletion_v2"))
		slog.Info("registered provider", "name", "minimax")
	}

	if cfg.Providers.Cohere.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("cohere", cfg.Providers.Cohere.APIKey, "https://api.cohere.ai/compatibility/v1", "command-a"))
		slog.Info("registered provider", "name", "cohere")
	}

	if cfg.Providers.Perplexity.APIKey != "" {
		registry.Register(providers.NewOpenAIProvider("perplexity", cfg.Providers.Perplexity.APIKey, "https://api.perplexity.ai", "sonar-pro"))
		slog.Info("registered provider", "name", "perplexity")
	}

	if cfg.Providers.DashScope.APIKey != "" {
		registry.Register(providers.NewDashScopeProvider("dashscope", cfg.Providers.DashScope.APIKey, cfg.Providers.DashScope.APIBase, "qwen3-max"))
		slog.Info("registered provider", "name", "dashscope")
	}

	if cfg.Providers.Bailian.APIKey != "" {
		base := cfg.Providers.Bailian.APIBase
		if base == "" {
			base = "https://coding-intl.dashscope.aliyuncs.com/v1"
		}
		registry.Register(providers.NewOpenAIProvider("bailian", cfg.Providers.Bailian.APIKey, base, "qwen3.5-plus"))
		slog.Info("registered provider", "name", "bailian")
	}

	if cfg.Providers.Zai.APIKey != "" {
		base := cfg.Providers.Zai.APIBase
		if base == "" {
			base = "https://api.z.ai/api/paas/v4"
		}
		registry.Register(providers.NewOpenAIProvider("zai", cfg.Providers.Zai.APIKey, base, "glm-5"))
		slog.Info("registered provider", "name", "zai")
	}

	if cfg.Providers.ZaiCoding.APIKey != "" {
		base := cfg.Providers.ZaiCoding.APIBase
		if base == "" {
			base = "https://api.z.ai/api/coding/paas/v4"
		}
		registry.Register(providers.NewOpenAIProvider("zai-coding", cfg.Providers.ZaiCoding.APIKey, base, "glm-5"))
		slog.Info("registered provider", "name", "zai-coding")
	}

	// Local / self-hosted Ollama — gated on Host, no API key required.
	// Ollama's OpenAI-compat endpoint accepts any non-empty Bearer value.
	if cfg.Providers.Ollama.Host != "" {
		host := cfg.Providers.Ollama.Host
		registry.Register(providers.NewOpenAIProvider("ollama", "ollama", host+"/v1", "llama3.3"))
		slog.Info("registered provider", "name", "ollama")
	}

	// Ollama Cloud — API key required (generate at ollama.com/settings/keys).
	if cfg.Providers.OllamaCloud.APIKey != "" {
		base := cfg.Providers.OllamaCloud.APIBase
		if base == "" {
			base = "https://ollama.com/v1"
		}
		registry.Register(providers.NewOpenAIProvider("ollama-cloud", cfg.Providers.OllamaCloud.APIKey, base, "llama3.3"))
		slog.Info("registered provider", "name", "ollama-cloud")
	}

	// Novita AI — OpenAI-compatible endpoint.
	if cfg.Providers.Novita.APIKey != "" {
		base := cfg.Providers.Novita.APIBase
		if base == "" {
			base = store.NovitaDefaultAPIBase
		}
		registry.Register(providers.NewOpenAIProvider("novita", cfg.Providers.Novita.APIKey, base, store.NovitaDefaultModel))
		slog.Info("registered provider", "name", "novita")
	}

	// BytePlus ModelArk — OpenAI-compatible (standard Bearer auth).
	if cfg.Providers.BytePlus.APIKey != "" {
		base := cfg.Providers.BytePlus.APIBase
		if base == "" {
			base = store.BytePlusDefaultAPIBase
		}
		prov := providers.NewOpenAIProvider("byteplus", cfg.Providers.BytePlus.APIKey, base, store.BytePlusDefaultModel)
		prov.WithProviderType(store.ProviderBytePlus)
		registry.Register(prov)
		slog.Info("registered provider", "name", "byteplus")
	}

	// BytePlus ModelArk Coding Plan — separate endpoint for developer tools quota.
	if cfg.Providers.BytePlusCoding.APIKey != "" {
		base := cfg.Providers.BytePlusCoding.APIBase
		if base == "" {
			base = store.BytePlusCodingDefaultAPIBase
		}
		prov := providers.NewOpenAIProvider("byteplus-coding", cfg.Providers.BytePlusCoding.APIKey, base, store.BytePlusDefaultModel)
		prov.WithProviderType(store.ProviderBytePlusCoding)
		registry.Register(prov)
		slog.Info("registered provider", "name", "byteplus-coding")
	}

	// Claude CLI provider (subscription-based, no API key needed)
	if cfg.Providers.ClaudeCLI.CLIPath != "" {
		cliPath := cfg.Providers.ClaudeCLI.CLIPath
		var opts []providers.ClaudeCLIOption
		if cfg.Providers.ClaudeCLI.Model != "" {
			opts = append(opts, providers.WithClaudeCLIModel(cfg.Providers.ClaudeCLI.Model))
		}
		if cfg.Providers.ClaudeCLI.BaseWorkDir != "" {
			opts = append(opts, providers.WithClaudeCLIWorkDir(cfg.Providers.ClaudeCLI.BaseWorkDir))
		}
		if cfg.Providers.ClaudeCLI.PermMode != "" {
			opts = append(opts, providers.WithClaudeCLIPermMode(cfg.Providers.ClaudeCLI.PermMode))
		}
		// Build per-session MCP config: external MCP servers + GoClaw bridge
		gatewayAddr := loopbackAddr(cfg.Gateway.Host, cfg.Gateway.Port)
		mcpData := providers.BuildCLIMCPConfigData(cfg.Tools.McpServers, gatewayAddr, cfg.Gateway.Token)
		opts = append(opts, providers.WithClaudeCLIMCPConfigData(mcpData))
		// Enable GoClaw security hooks (shell deny patterns, path restrictions)
		opts = append(opts, providers.WithClaudeCLISecurityHooks(
			cfg.Providers.ClaudeCLI.BaseWorkDir, true))
		registry.Register(providers.NewClaudeCLIProvider(cliPath, opts...))
		slog.Info("registered provider", "name", "claude-cli")
	}

	// ACP provider (config-based) — orchestrates any ACP-compatible agent binary
	if cfg.Providers.ACP.Binary != "" {
		registerACPFromConfig(registry, cfg.Providers.ACP)
	}

	// Detect and register CLI-based providers (auto-install if configured but missing).
	registerCLIProviders(registry, cfg, installer)
}

// buildMCPServerLookup creates an MCPServerLookup from an MCPServerStore.
// Returns nil if mcpStore is nil.
func buildMCPServerLookup(mcpStore store.MCPServerStore) providers.MCPServerLookup {
	if mcpStore == nil {
		return nil
	}
	return func(ctx context.Context, agentID string) []providers.MCPServerEntry {
		aid, err := uuid.Parse(agentID)
		if err != nil {
			return nil
		}
		accessible, err := mcpStore.ListAccessible(ctx, aid, "")
		if err != nil {
			slog.Warn("claude-cli: failed to list agent MCP servers", "agent_id", agentID, "error", err)
			return nil
		}
		var entries []providers.MCPServerEntry
		for _, info := range accessible {
			srv := info.Server
			if !srv.Enabled {
				continue
			}
			entry := providers.MCPServerEntry{
				Name:      srv.Name,
				Transport: srv.Transport,
				Command:   srv.Command,
				URL:       srv.URL,
				Args:      jsonToStringSlice(srv.Args),
				Headers:   jsonToStringMap(srv.Headers),
				Env:       jsonToStringMap(srv.Env),
			}
			entries = append(entries, entry)
		}
		return entries
	}
}

// jsonToStringSlice converts a json.RawMessage to []string.
func jsonToStringSlice(data json.RawMessage) []string {
	if len(data) == 0 {
		return nil
	}
	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

// jsonToStringMap converts a json.RawMessage to map[string]string.
func jsonToStringMap(data json.RawMessage) map[string]string {
	if len(data) == 0 {
		return nil
	}
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

// registerProvidersFromDB loads providers from Postgres and registers them.
// DB providers are registered after config providers, so they take precedence (overwrite).
// gatewayAddr is used to inject GoClaw MCP bridge for Claude CLI providers.
// mcpStore is optional; when provided, per-agent MCP servers are injected into CLI config.
// cfg provides fallback api_base values from config/env when DB providers have none set.
func registerProvidersFromDB(registry *providers.Registry, provStore store.ProviderStore, secretStore store.ConfigSecretsStore, gatewayAddr, gatewayToken string, mcpStore store.MCPServerStore, cfg *config.Config, modelReg providers.ModelRegistry, installer *cliinstall.Installer) {
	dbProviders, err := provStore.ListAllProviders(context.Background())
	if err != nil {
		slog.Warn("failed to load providers from DB", "error", err)
		return
	}
	for _, p := range dbProviders {
		// Claude CLI doesn't need API key
		if !p.Enabled {
			continue
		}
		if p.ProviderType == store.ProviderClaudeCLI {
			cliPath := p.APIBase // reuse APIBase field for CLI path
			if cliPath == "" {
				cliPath = "claude"
			}
			// Validate: only accept "claude" or absolute path
			if cliPath != "claude" && !filepath.IsAbs(cliPath) {
				slog.Warn("security.claude_cli: invalid path from DB, using default", "path", cliPath)
				cliPath = "claude"
			}
		// Auto-install if binary not found (only with installer available).
		if _, err := exec.LookPath(cliPath); err != nil {
			if installer != nil {
				spec := cliinstall.CLISpec{ProviderType: "claude_cli", Binary: "claude", Owner: "anthropics", Repo: "claude-code", DefaultModel: "sonnet"}
				result, ierr := installer.Install(context.Background(), spec)
				if ierr != nil {
					slog.Warn("claude-cli: binary not found and install failed, skipping", "path", cliPath, "error", ierr)
					continue
				}
				cliPath = result.Path
				slog.Info("claude-cli: auto-installed from DB config", "path", cliPath, "version", result.Version)
			} else {
				slog.Warn("claude-cli: binary not found, skipping", "path", cliPath, "error", err)
				continue
			}
		}
			var cliOpts []providers.ClaudeCLIOption
			cliOpts = append(cliOpts, providers.WithClaudeCLIName(p.Name))
			cliOpts = append(cliOpts, providers.WithClaudeCLISecurityHooks("", true))
			if gatewayAddr != "" {
				mcpData := providers.BuildCLIMCPConfigData(nil, gatewayAddr, gatewayToken)
				mcpData.AgentMCPLookup = buildMCPServerLookup(mcpStore)
				cliOpts = append(cliOpts, providers.WithClaudeCLIMCPConfigData(mcpData))
			}
			registry.RegisterForTenant(p.TenantID, providers.NewClaudeCLIProvider(cliPath, cliOpts...))
			slog.Info("registered provider from DB", "name", p.Name)
			continue
		}
		// ACP provider — no API key needed (agents manage their own auth).
		if p.ProviderType == store.ProviderACP {
			registerACPFromDB(registry, p)
			continue
		}
		// New CLI-based providers (codex_cli, copilot, opencode) — no API key needed.
		if p.ProviderType == store.ProviderCodexCLI || p.ProviderType == store.ProviderCopilot || p.ProviderType == store.ProviderOpenCode {
			registerCLIFromDB(registry, installer, p)
			continue
		}

		// Local Ollama requires no API key — handle before the key guard (same pattern as ClaudeCLI).
		// api_base is stored with /v1 (normalized at write time), so no suffix appending needed.
		if p.ProviderType == store.ProviderOllama {
			host := p.APIBase
			if host == "" {
				host = "http://localhost:11434/v1"
			}
			registry.RegisterForTenant(p.TenantID, providers.NewOpenAIProvider(p.Name, "ollama", config.DockerLocalhost(host), "llama3.3"))
			slog.Info("registered provider from DB", "name", p.Name)
			continue
		}

		if p.APIKey == "" {
			continue
		}
		// Fall back to config/env api_base when DB provider has none set.
		if p.APIBase == "" && cfg != nil {
			if base := cfg.Providers.APIBaseForType(p.ProviderType); base != "" {
				p.APIBase = base
				slog.Info("provider api_base inherited from config", "name", p.Name, "api_base", base)
			}
		}
		switch p.ProviderType {
		case store.ProviderChatGPTOAuth:
			ts := oauth.NewDBTokenSource(provStore, secretStore, p.Name).WithTenantID(p.TenantID)
			codex := providers.NewCodexProvider(p.Name, ts, p.APIBase, "")
			if oauthSettings := store.ParseChatGPTOAuthProviderSettings(p.Settings); oauthSettings != nil {
				codex.WithRoutingDefaults(oauthSettings.CodexPool.Strategy, oauthSettings.CodexPool.ExtraProviderNames)
			}
			registry.RegisterForTenant(p.TenantID, codex)
		case store.ProviderAnthropicNative:
			registry.RegisterForTenant(p.TenantID, providers.NewAnthropicProvider(p.APIKey,
				providers.WithAnthropicName(p.Name),
				providers.WithAnthropicBaseURL(p.APIBase),
				providers.WithAnthropicRegistry(modelReg)))
		case store.ProviderDashScope:
			registry.RegisterForTenant(p.TenantID, providers.NewDashScopeProvider(p.Name, p.APIKey, p.APIBase, ""))
		case store.ProviderBailian:
			base := p.APIBase
			if base == "" {
				base = "https://coding-intl.dashscope.aliyuncs.com/v1"
			}
			registry.RegisterForTenant(p.TenantID, providers.NewOpenAIProvider(p.Name, p.APIKey, base, "qwen3.5-plus"))
		case store.ProviderZai:
			base := p.APIBase
			if base == "" {
				base = "https://api.z.ai/api/paas/v4"
			}
			registry.RegisterForTenant(p.TenantID, providers.NewOpenAIProvider(p.Name, p.APIKey, base, "glm-5"))
		case store.ProviderZaiCoding:
			base := p.APIBase
			if base == "" {
				base = "https://api.z.ai/api/coding/paas/v4"
			}
			registry.RegisterForTenant(p.TenantID, providers.NewOpenAIProvider(p.Name, p.APIKey, base, "glm-5"))
		case store.ProviderOllamaCloud:
			base := p.APIBase
			if base == "" {
				base = "https://ollama.com/v1"
			}
			registry.RegisterForTenant(p.TenantID, providers.NewOpenAIProvider(p.Name, p.APIKey, base, "llama3.3"))
		case store.ProviderNovita:
			base := p.APIBase
			if base == "" {
				base = store.NovitaDefaultAPIBase
			}
			registry.RegisterForTenant(p.TenantID, providers.NewOpenAIProvider(p.Name, p.APIKey, base, store.NovitaDefaultModel))
		case store.ProviderBytePlus:
			base := p.APIBase
			if base == "" {
				base = store.BytePlusDefaultAPIBase
			}
			prov := providers.NewOpenAIProvider(p.Name, p.APIKey, base, store.BytePlusDefaultModel)
			prov.WithProviderType(p.ProviderType)
			registry.RegisterForTenant(p.TenantID, prov)
		case store.ProviderBytePlusCoding:
			base := p.APIBase
			if base == "" {
				base = store.BytePlusCodingDefaultAPIBase
			}
			prov := providers.NewOpenAIProvider(p.Name, p.APIKey, base, store.BytePlusDefaultModel)
			prov.WithProviderType(p.ProviderType)
			registry.RegisterForTenant(p.TenantID, prov)
		default:
			prov := providers.NewOpenAIProvider(p.Name, p.APIKey, p.APIBase, "")
			prov.WithProviderType(p.ProviderType)
			if p.ProviderType == store.ProviderMiniMax {
				prov.WithChatPath("/text/chatcompletion_v2")
			}
			if p.ProviderType == store.ProviderOpenRouter {
				prov.WithSiteInfo("https://goclaw.sh", "GoClaw")
			}
			registry.RegisterForTenant(p.TenantID, prov)
		}
		slog.Info("registered provider from DB", "name", p.Name)
	}
}

// registerACPFromConfig registers an ACP provider from config file settings.
func registerACPFromConfig(registry *providers.Registry, cfg config.ACPConfig) {
	if _, err := exec.LookPath(cfg.Binary); err != nil {
		slog.Warn("acp: binary not found, skipping", "binary", cfg.Binary, "error", err)
		return
	}
	idleTTL := 5 * time.Minute
	if cfg.IdleTTL != "" {
		if d, err := time.ParseDuration(cfg.IdleTTL); err == nil {
			idleTTL = d
		}
	}
	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = defaultACPWorkDir()
	}
	var opts []providers.ACPOption
	if cfg.Model != "" {
		opts = append(opts, providers.WithACPModel(cfg.Model))
	}
	if cfg.PermMode != "" {
		opts = append(opts, providers.WithACPPermMode(cfg.PermMode))
	}
	registry.Register(providers.NewACPProvider(
		cfg.Binary, cfg.Args, workDir, idleTTL, tools.DefaultDenyPatterns(), opts...,
	))
	slog.Info("registered provider", "name", "acp", "binary", cfg.Binary)
}

// registerACPFromDB registers an ACP provider from a DB provider row.
func registerACPFromDB(registry *providers.Registry, p store.LLMProviderData) {
	binary := p.APIBase // repurpose api_base as binary path
	if binary == "" {
		slog.Warn("acp: no binary specified in DB provider", "name", p.Name)
		return
	}
	if binary != "claude" && binary != "codex" && binary != "gemini" && !filepath.IsAbs(binary) {
		slog.Warn("security.acp: invalid binary path from DB", "path", binary)
		return
	}
	if _, err := exec.LookPath(binary); err != nil {
		slog.Warn("acp: binary not found, skipping", "binary", binary, "error", err)
		return
	}
	// Parse settings JSONB for extra config
	var settings struct {
		Args     []string `json:"args"`
		IdleTTL  string   `json:"idle_ttl"`
		PermMode string   `json:"perm_mode"`
		WorkDir  string   `json:"work_dir"`
	}
	if p.Settings != nil {
		if err := json.Unmarshal(p.Settings, &settings); err != nil {
			slog.Warn("acp: invalid settings JSON, using defaults", "name", p.Name, "error", err)
		}
	}
	idleTTL := 5 * time.Minute
	if settings.IdleTTL != "" {
		if d, err := time.ParseDuration(settings.IdleTTL); err == nil {
			idleTTL = d
		}
	}
	workDir := settings.WorkDir
	if workDir == "" {
		workDir = defaultACPWorkDir()
	}
	registry.RegisterForTenant(p.TenantID, providers.NewACPProvider(
		binary, settings.Args, workDir, idleTTL, tools.DefaultDenyPatterns(),
		providers.WithACPName(p.Name),
		providers.WithACPModel(p.Name),
	))
	slog.Info("registered provider from DB", "name", p.Name, "type", "acp")
}

// defaultACPWorkDir returns the default workspace directory for ACP agents.
func defaultACPWorkDir() string {
	return filepath.Join(config.ResolvedDataDirFromEnv(), "acp-workspaces")
}

// newCLIInstaller creates a cliinstall.Installer from config and env vars.
// Creates its own GitHubClient, independent of the shared skills installer.
func newCLIInstaller(cfg *config.Config) *cliinstall.Installer {
	token := os.Getenv("GOCLAW_PACKAGES_GITHUB_TOKEN")
	binDir := resolveCLIBinDir(cfg)
	var allowedOrgs []string
	if v := os.Getenv("GOCLAW_PACKAGES_GITHUB_ALLOWED_ORGS"); v != "" {
		for o := range strings.SplitSeq(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowedOrgs = append(allowedOrgs, o)
			}
		}
	}
	return cliinstall.NewInstaller(token, binDir, allowedOrgs)
}

// resolveCLIBinDir determines the binary install directory.
// Priority: GOCLAW_CLI_BIN_DIR > OS-specific default.
func resolveCLIBinDir(cfg *config.Config) string {
	if v := os.Getenv("GOCLAW_CLI_BIN_DIR"); v != "" {
		return v
	}
	return filepath.Join(config.ResolvedDataDirFromEnv(), ".runtime", "bin")
}

// registerCLIProviders detects and registers CLI-based LLM providers.
// - If configured explicitly (in config), tries LookPath → auto-install → register.
// - If detected but not configured, logs info only (no auto-registration).
// - If neither detected nor configured, silent.
func registerCLIProviders(registry *providers.Registry, cfg *config.Config, installer *cliinstall.Installer) {
	for _, spec := range cliinstall.KnownCLIs {
		configured := isCLIConfigured(cfg, spec)
		foundPath, found := cliinstall.Detect(spec.Binary)

		switch {
		case configured:
			if !found {
				slog.Info("cliinstall: configured CLI not found, attempting install",
					"binary", spec.Binary, "provider_type", spec.ProviderType)
				result, err := installer.Install(context.Background(), spec)
				if err != nil {
					slog.Error("cliinstall: failed to install configured CLI",
						"binary", spec.Binary, "error", err)
					continue
				}
				foundPath = result.Path
				slog.Info("cliinstall: installed and registered",
					"binary", spec.Binary, "version", result.Version)
			}
			registerCLIProvider(registry, spec, foundPath)
		case found:
			cliPath := cliPathFromConfig(cfg, spec)
			if cliPath == "" {
				cliPath = foundPath
			}
			slog.Info("cli available but not configured",
				"binary", spec.Binary,
				"path", cliPath,
				"provider_type", spec.ProviderType,
				"hint", "add a provider in config or DB to enable")
		}
		// case !configured && !found: silent
	}
}

// isCLIConfigured checks if a CLI is explicitly configured in config file.
func isCLIConfigured(cfg *config.Config, spec cliinstall.CLISpec) bool {
	switch spec.ProviderType {
	case "claude_cli":
		return cfg.Providers.ClaudeCLI.CLIPath != ""
	case "codex_cli":
		return cfg.Providers.CodexCLI.CLIPath != ""
	case "copilot":
		return cfg.Providers.Copilot.CLIPath != ""
	case "opencode":
		return cfg.Providers.OpenCode.CLIPath != ""
	}
	return false
}

// cliPathFromConfig returns the configured cli_path for a CLI spec.
func cliPathFromConfig(cfg *config.Config, spec cliinstall.CLISpec) string {
	switch spec.ProviderType {
	case "claude_cli":
		return cfg.Providers.ClaudeCLI.CLIPath
	case "codex_cli":
		return cfg.Providers.CodexCLI.CLIPath
	case "copilot":
		return cfg.Providers.Copilot.CLIPath
	case "opencode":
		return cfg.Providers.OpenCode.CLIPath
	}
	return ""
}

// registerCLIProvider creates the appropriate provider for a CLI spec and registers it.
func registerCLIProvider(registry *providers.Registry, spec cliinstall.CLISpec, binaryPath string) {
	switch spec.ProviderType {
	case "claude_cli":
		// Claude CLI uses ClaudeCLIProvider with its specific options.
		var opts []providers.ClaudeCLIOption
		opts = append(opts, providers.WithClaudeCLIModel(spec.DefaultModel))
		opts = append(opts, providers.WithClaudeCLISecurityHooks("", true))
		registry.Register(providers.NewClaudeCLIProvider(binaryPath, opts...))
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

// registerCLIFromDB resolves and registers a CLI-based provider from a DB row.
// Uses auto-install if the binary is not found and an installer is available.
func registerCLIFromDB(registry *providers.Registry, installer *cliinstall.Installer, p store.LLMProviderData) {
	cliPath := p.APIBase
	if cliPath == "" {
		cliPath = p.Name // fallback to provider name as binary name
	}
	// Validate: only accept simple names or absolute paths.
	if cliPath != filepath.Base(cliPath) && !filepath.IsAbs(cliPath) {
		slog.Warn("security.cli: invalid path from DB, using name as default", "path", cliPath, "name", p.Name)
		cliPath = p.ProviderType // e.g. "codex_cli" -> binary is "codex"
	}

	// Resolve binary name from provider type for detection/install fallback.
	binaryName := resolveCLIBinaryName(p.ProviderType)
	spec := cliinstall.CLISpec{ProviderType: p.ProviderType, Binary: binaryName}

	// Try LookPath, then auto-install if available.
	if foundPath, found := cliinstall.Detect(binaryName); found {
		cliPath = foundPath
	} else if _, err := exec.LookPath(cliPath); err != nil {
		if installer != nil {
			result, ierr := installer.Install(context.Background(), spec)
			if ierr != nil {
				slog.Warn("cli: binary not found and install failed, skipping",
					"provider_type", p.ProviderType, "name", p.Name, "error", ierr)
				return
			}
			cliPath = result.Path
			slog.Info("cli: auto-installed from DB config",
				"provider_type", p.ProviderType, "path", cliPath, "version", result.Version)
		} else {
			slog.Warn("cli: binary not found, skipping",
				"provider_type", p.ProviderType, "path", cliPath, "error", err)
			return
		}
	}

	registerCLIProvider(registry, spec, cliPath)
}

// resolveCLIBinaryName maps provider type to binary name.
func resolveCLIBinaryName(providerType string) string {
	switch providerType {
	case "claude_cli":
		return "claude"
	case "codex_cli":
		return "codex"
	case "copilot":
		return "copilot"
	case "opencode":
		return "opencode"
	}
	return providerType
}
