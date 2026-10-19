package groq

import (
	"context"
	"errors"
	"strings"

	oai "github.com/sashabaranov/go-openai"
)

// APIBaseURL is Groq's OpenAI-compatible endpoint.
const APIBaseURL = "https://api.groq.com/openai/v1"

// DefaultModel is used when GenerateContent is called with an empty model name.
const DefaultModel = "llama-3.3-70b-versatile"

// GenerateContent calls Groq's chat completions API and returns the assistant message text.
func GenerateContent(ctx context.Context, apiKey, model, userInput string, searchMode bool) (string, error) {
	if apiKey == "" {
		return "", errors.New("groq: empty API key")
	}
	if userInput == "" {
		return "", errors.New("groq: empty user input")
	}
	model = strings.TrimSpace(model)
	reqContent := userInput

	if searchMode {
		if model == "" {
			model = "groq/compound"
		}
		if !strings.HasPrefix(model, "groq/compound") {
			reqContent = userInput + "\n\n[Note: Real-time web search mode is ENABLED. Please provide accurate, real-time, and up-to-date information for current events, rankings, and statistics.]"
		}
	} else {
		if !strings.HasPrefix(model, "groq/compound") {
			model = strings.TrimPrefix(model, "groq/")
		}
		if model == "" {
			model = DefaultModel
		}
	}

	cfg := oai.DefaultConfig(apiKey)
	cfg.BaseURL = APIBaseURL
	client := oai.NewClientWithConfig(cfg)

	resp, err := client.CreateChatCompletion(ctx, oai.ChatCompletionRequest{
		Model: model,
		Messages: []oai.ChatCompletionMessage{
			{Role: oai.ChatMessageRoleUser, Content: reqContent},
		},
		MaxTokens: 8192,
	})
	if err != nil && searchMode && strings.HasPrefix(model, "groq/compound") {
		// Fallback to DefaultModel if groq/compound is temporarily unavailable
		model = DefaultModel
		reqContent = userInput + "\n\n[Note: Real-time web search mode is ENABLED. Please provide accurate, real-time, and up-to-date information for current events, rankings, and statistics.]"
		resp, err = client.CreateChatCompletion(ctx, oai.ChatCompletionRequest{
			Model: model,
			Messages: []oai.ChatCompletionMessage{
				{Role: oai.ChatMessageRoleUser, Content: reqContent},
			},
			MaxTokens: 8192,
		})
	}
	if err != nil {
		return "", wrapChatCompletionError(err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("groq: empty choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
