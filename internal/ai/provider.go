package ai

// Provider selects which backend GenerateWith uses.
type Provider string

const (
	ProviderGemini  Provider = "gemini"
	ProviderOpenAI  Provider = "openai"
	ProviderGroq    Provider = "groq"
	ProviderClaude  Provider = "claude"
	ProviderOpenCode      Provider = "opencode"
	ProviderOpenCodeLocal Provider = "opencode_local"
)
