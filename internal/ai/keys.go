package ai

import "github.com/Bebii-Digital/bebii-blog/internal/entity"

// Keys holds API credentials and per-provider options. Add fields here when wiring
// additional models (OpenAI, Anthropic, etc.).
type Keys struct {
	Gemini       string
	GeminiModel  string // empty = provider default (see gemini.DefaultModel)
	GeminiSearch bool
	OpenAI       string
	OpenAIModel  string // empty = provider default (see openai.DefaultModel)
	OpenAISearch bool
	Groq         string
	GroqModel    string // empty = provider default (see groq.DefaultModel)
	GroqSearch   bool
	Claude       string
	ClaudeModel  string // empty = provider default (see claude.DefaultModel)
	ClaudeSearch bool
	OpenCode       string
	OpenCodeModel  string // empty = provider default (see opencode.DefaultModel)
	OpenCodeSearch bool
	OpenCodeLocal  entity.OpenCodeLocalSetting
}
