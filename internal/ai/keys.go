package ai

// Keys holds API credentials and per-provider options. Add fields here when wiring
// additional models (OpenAI, Anthropic, etc.).
type Keys struct {
	Gemini      string
	GeminiModel string // empty = provider default (see gemini.DefaultModel)
	OpenAI      string
	OpenAIModel string // empty = provider default (see openai.DefaultModel)
	Groq        string
	GroqModel   string // empty = provider default (see groq.DefaultModel)
	Claude      string
	ClaudeModel string // empty = provider default (see claude.DefaultModel)
}
