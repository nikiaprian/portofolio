package ai

import (
	"context"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
)

// KeysProvider supplies API keys for [Generator] (e.g. from ai_setting or tests).
type KeysProvider interface {
	Keys(ctx context.Context) (Keys, error)
}

type staticKeysProvider struct {
	keys Keys
}

func (s staticKeysProvider) Keys(ctx context.Context) (Keys, error) {
	_ = ctx
	return s.keys, nil
}

// StaticKeys returns a KeysProvider that always returns the same keys (tests / simple wiring).
func StaticKeys(keys Keys) KeysProvider {
	return staticKeysProvider{keys: keys}
}

type repoKeysProvider struct {
	repo repositories.AISettingRepository
}

func (r *repoKeysProvider) Keys(ctx context.Context) (Keys, error) {
	_ = ctx
	row, err := r.repo.GetOrCreate()
	if err != nil {
		return Keys{}, err
	}
	return KeysFromEntity(row), nil
}

// NewRepoKeysProvider loads keys from the ai_setting table on each call.
func NewRepoKeysProvider(repo repositories.AISettingRepository) KeysProvider {
	return &repoKeysProvider{repo: repo}
}

// KeysFromEntity maps a database row into Keys for providers.
func KeysFromEntity(a *entity.AISetting) Keys {
	if a == nil {
		return Keys{}
	}
	return Keys{
		Gemini:      a.Gemini,
		GeminiModel: a.GeminiModel,
		OpenAI:      a.OpenAI,
		OpenAIModel: a.OpenAIModel,
		Groq:        a.Groq,
		GroqModel:   a.GroqModel,
		Claude:      a.Claude,
		ClaudeModel: a.ClaudeModel,
	}
}
