package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/nikiaprian/tirtayasa/internal/ai/claude"
	"github.com/nikiaprian/tirtayasa/internal/ai/gemini"
	"github.com/nikiaprian/tirtayasa/internal/ai/groq"
	"github.com/nikiaprian/tirtayasa/internal/ai/openai"
)

var (
	// ErrMissingAPIKey means no key is configured for the requested provider.
	ErrMissingAPIKey = errors.New("ai: API key not configured for provider")
	// ErrUnsupportedProvider is returned when Provider is unknown or not wired yet.
	ErrUnsupportedProvider = errors.New("ai: unsupported provider")
	// ErrAutoProviderUnavailable means auto mode cannot find a working provider.
	ErrAutoProviderUnavailable = errors.New("ai: auto provider unavailable")
	// ErrNoImageProviderKey means cover-image generation has no usable key (e.g. auto/OpenAI path without OpenAI key).
	ErrNoImageProviderKey = errors.New("ai: no API key configured for image generation")
)

// Generator runs AI-backed flows (e.g. blog draft text) using configured keys.
type Generator struct {
	keys KeysProvider
}

// NewGenerator returns a generator that loads keys from the given provider (e.g. [NewRepoKeysProvider]).
func NewGenerator(keys KeysProvider) *Generator {
	return &Generator{keys: keys}
}

func (g *Generator) loadKeys(ctx context.Context) (Keys, error) {
	if g.keys == nil {
		return Keys{}, nil
	}
	return g.keys.Keys(ctx)
}

// GenerateBlogContent generates text suitable as a blog body or draft using auto
// fallback: it tries each configured provider in order and returns the first success.
func (g *Generator) GenerateBlogContent(ctx context.Context, prompt string) (string, error) {
	text, _, err := g.GenerateBlogContentWithProvider(ctx, prompt)
	return text, err
}

// GenerateBlogContentWithProvider is like [Generator.GenerateBlogContent] but also returns
// which provider produced the output (empty Provider on error).
func (g *Generator) GenerateBlogContentWithProvider(ctx context.Context, prompt string) (string, Provider, error) {
	keys, err := g.loadKeys(ctx)
	if err != nil {
		return "", "", err
	}

	type candidate struct {
		provider Provider
		key      string
	}
	// Order: try Gemini and Groq before OpenAI so a temporary OpenAI/Cloudflare 502 does not block
	// users who also configured another key (auto still falls through the full list).
	candidates := []candidate{
		{provider: ProviderGemini, key: keys.Gemini},
		{provider: ProviderGroq, key: keys.Groq},
		{provider: ProviderOpenAI, key: keys.OpenAI},
		{provider: ProviderClaude, key: keys.Claude},
	}

	var lastErr error
	tried := 0
	for _, c := range candidates {
		if c.key == "" {
			continue
		}
		tried++
		text, genErr := g.generateWithKeys(ctx, keys, c.provider, prompt)
		if genErr == nil {
			return text, c.provider, nil
		}
		lastErr = genErr
	}

	if tried == 0 {
		return "", "", fmt.Errorf("%w: no API key configured", ErrAutoProviderUnavailable)
	}
	if lastErr != nil {
		if isAIQuotaOrRateLimitErr(lastErr) {
			return "", "", lastErr
		}
		return "", "", fmt.Errorf("%w: %v", ErrAutoProviderUnavailable, lastErr)
	}
	return "", "", ErrAutoProviderUnavailable
}

// GenerateWith calls the selected provider. Extend the switch as you add providers.
func (g *Generator) GenerateWith(ctx context.Context, p Provider, prompt string) (string, error) {
	keys, err := g.loadKeys(ctx)
	if err != nil {
		return "", err
	}
	return g.generateWithKeys(ctx, keys, p, prompt)
}

// HasProviderKey reports whether a key exists for the selected provider.
func (g *Generator) HasProviderKey(ctx context.Context, p Provider) (bool, error) {
	keys, err := g.loadKeys(ctx)
	if err != nil {
		return false, err
	}
	switch p {
	case ProviderGemini:
		return keys.Gemini != "", nil
	case ProviderOpenAI:
		return keys.OpenAI != "", nil
	case ProviderGroq:
		return keys.Groq != "", nil
	case ProviderClaude:
		return keys.Claude != "", nil
	default:
		return false, fmt.Errorf("%w: %q", ErrUnsupportedProvider, p)
	}
}

// HasAnyKey reports whether at least one provider key exists.
func (g *Generator) HasAnyKey(ctx context.Context) (bool, error) {
	keys, err := g.loadKeys(ctx)
	if err != nil {
		return false, err
	}
	return keys.OpenAI != "" || keys.Groq != "" || keys.Gemini != "" || keys.Claude != "", nil
}

func isAIQuotaOrRateLimitErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, openai.ErrQuotaOrRateLimit) ||
		errors.Is(err, groq.ErrQuotaOrRateLimit) ||
		errors.Is(err, gemini.ErrQuotaOrRateLimit) ||
		errors.Is(err, claude.ErrQuotaOrRateLimit)
}

func (g *Generator) generateWithKeys(ctx context.Context, keys Keys, p Provider, prompt string) (string, error) {
	if prompt == "" {
		return "", errors.New("ai: empty prompt")
	}

	switch p {
	case ProviderGemini:
		if keys.Gemini == "" {
			return "", fmt.Errorf("%w: gemini", ErrMissingAPIKey)
		}
		return gemini.GenerateContent(ctx, keys.Gemini, keys.GeminiModel, prompt)
	case ProviderOpenAI:
		if keys.OpenAI == "" {
			return "", fmt.Errorf("%w: openai", ErrMissingAPIKey)
		}
		return openai.GenerateContent(ctx, keys.OpenAI, keys.OpenAIModel, prompt)
	case ProviderGroq:
		if keys.Groq == "" {
			return "", fmt.Errorf("%w: groq", ErrMissingAPIKey)
		}
		return groq.GenerateContent(ctx, keys.Groq, keys.GroqModel, prompt)
	case ProviderClaude:
		if keys.Claude == "" {
			return "", fmt.Errorf("%w: claude", ErrMissingAPIKey)
		}
		return claude.GenerateContent(ctx, keys.Claude, keys.ClaudeModel, prompt)
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedProvider, p)
	}
}
