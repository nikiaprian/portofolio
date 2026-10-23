package gemini

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"
)

// DefaultModel is used when GenerateContent is called with an empty model name.
const DefaultModel = "gemini-3-flash-preview"

// GenerateContent calls the Gemini API and returns the model's primary text response.
func GenerateContent(ctx context.Context, apiKey, model, userInput string) (string, error) {
	if apiKey == "" {
		return "", errors.New("gemini: empty API key")
	}
	if userInput == "" {
		return "", errors.New("gemini: empty user input")
	}
	if model == "" {
		model = DefaultModel
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  apiKey,
	})
	if err != nil {
		return "", fmt.Errorf("gemini client: %w", err)
	}

	// High thinking consumes the same max-output budget as visible text, which caps
	// long articles (~2k words). Prefer minimal thinking + explicit token budget for CMS drafts.
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: 8192,
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel: genai.ThinkingLevelMinimal,
		},
	}

	contents := []*genai.Content{
		genai.NewContentFromText(userInput, genai.RoleUser),
	}

	result, err := client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return "", WrapGenerateError(err)
	}
	return result.Text(), nil
}
