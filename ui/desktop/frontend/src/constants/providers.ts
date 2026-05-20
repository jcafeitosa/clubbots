export interface ProviderTypeInfo {
  value: string
  label: string
  apiBase: string
}

/** Provider types that don't require an API key (subprocess/local). */
export const NO_API_KEY_TYPES = new Set([
  "ollama",
  "claude_cli",
  "codex_cli",
  "copilot",
  "opencode",
  "acp",
]);

export const PROVIDER_TYPES: ProviderTypeInfo[] = [
  { value: "chatgpt_oauth", label: "ChatGPT Subscription (OAuth)", apiBase: "" },
  { value: "anthropic_native", label: "Anthropic (Native)", apiBase: "https://api.anthropic.com" },
  { value: "openai_compat", label: "OpenAI Compatible", apiBase: "https://api.openai.com/v1" },
  { value: "gemini_native", label: "Google Gemini", apiBase: "https://generativelanguage.googleapis.com/v1beta/openai" },
  { value: "openrouter", label: "OpenRouter", apiBase: "https://openrouter.ai/api/v1" },
  { value: "groq", label: "Groq", apiBase: "https://api.groq.com/openai/v1" },
  { value: "deepseek", label: "DeepSeek", apiBase: "https://api.deepseek.com/v1" },
  { value: "mistral", label: "Mistral AI", apiBase: "https://api.mistral.ai/v1" },
  { value: "xai", label: "xAI (Grok)", apiBase: "https://api.x.ai/v1" },
  { value: "minimax_native", label: "MiniMax (Native)", apiBase: "https://api.minimax.io/v1" },
  { value: "novita", label: "Novita AI", apiBase: "https://api.novita.ai/openai" },
  { value: "cohere", label: "Cohere", apiBase: "https://api.cohere.ai/compatibility/v1" },
  { value: "perplexity", label: "Perplexity", apiBase: "https://api.perplexity.ai" },
  { value: "dashscope", label: "DashScope (Qwen)", apiBase: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1" },
  { value: "bailian", label: "Bailian Coding", apiBase: "https://coding-intl.dashscope.aliyuncs.com/v1" },
  { value: "yescale", label: "YesScale", apiBase: "https://api.yescale.one/v1" },
  { value: "zai", label: "Z.ai API", apiBase: "https://api.z.ai/api/paas/v4" },
  { value: "zai_coding", label: "Z.ai Coding Plan", apiBase: "https://api.z.ai/api/coding/paas/v4" },
  { value: "byteplus", label: "BytePlus ModelArk", apiBase: "https://ark.ap-southeast.bytepluses.com/api/v3" },
  { value: "byteplus_coding", label: "BytePlus Coding Plan", apiBase: "https://ark.ap-southeast.bytepluses.com/api/coding/v3" },
  { value: "ollama", label: "Ollama (Local)", apiBase: "http://localhost:11434/v1" },
  { value: "ollama_cloud", label: "Ollama Cloud", apiBase: "https://ollama.com/v1" },
  { value: "claude_cli", label: "Claude CLI (Local)", apiBase: "" },
  { value: "codex_cli", label: "Codex CLI (Local)", apiBase: "" },
  { value: "copilot", label: "GitHub Copilot CLI (Local)", apiBase: "" },
  { value: "opencode", label: "OpenCode CLI (Local)", apiBase: "" },
  { value: "acp", label: "ACP Agent (Subprocess)", apiBase: "" },
  { value: "voyage", label: "Voyage AI (Embeddings)", apiBase: "https://api.voyageai.com/v1" },
];
