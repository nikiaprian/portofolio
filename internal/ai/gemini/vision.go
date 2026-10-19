package gemini

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"
)

// GenerateContentWithImage calls Gemini with a text prompt and one inline image.
func GenerateContentWithImage(ctx context.Context, apiKey, model, userInput, mime string, imageRaw []byte, searchMode bool) (string, error) {
	if apiKey == "" {
		return "", errors.New("gemini: empty API key")
	}
	if userInput == "" {
		return "", errors.New("gemini: empty user input")
	}
	if len(imageRaw) == 0 {
		return "", errors.New("gemini: empty image data")
	}
	if mime == "" {
		mime = "image/png"
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

	config := &genai.GenerateContentConfig{
		MaxOutputTokens: 8192,
	}

	if searchMode {
		config.Tools = []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		}
	} else {
		config.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingLevel: genai.ThinkingLevelMinimal,
		}
	}

	parts := []*genai.Part{
		genai.NewPartFromBytes(imageRaw, mime),
		genai.NewPartFromText(userInput),
	}
	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, err := client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil && searchMode {
		config.Tools = nil
		config.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingLevel: genai.ThinkingLevelMinimal,
		}
		fallbackInput := userInput + "\n\n[Catatan: Mode pencarian web real-time AKTIF. Berikan informasi yang akurat, terbarui, dan faktual tentang berita, peristiwa, dan data terkini.]"
		fallbackParts := []*genai.Part{
			genai.NewPartFromBytes(imageRaw, mime),
			genai.NewPartFromText(fallbackInput),
		}
		contents = []*genai.Content{
			genai.NewContentFromParts(fallbackParts, genai.RoleUser),
		}
		result, err = client.Models.GenerateContent(ctx, model, contents, config)
	}
	if err != nil {
		return "", WrapGenerateError(err)
	}
	return result.Text(), nil
}
