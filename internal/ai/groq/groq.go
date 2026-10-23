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
func GenerateContent(ctx context.Context, apiKey, model, userInput string) (string, error) {
	if apiKey == "" {
		return "", errors.New("groq: empty API key")
	}
	if userInput == "" {
		return "", errors.New("groq: empty user input")
	}
	model = strings.TrimSpace(model)
	model = strings.TrimPrefix(model, "groq/")
	if model == "" {
		model = DefaultModel
	}

	cfg := oai.DefaultConfig(apiKey)
	cfg.BaseURL = APIBaseURL
	client := oai.NewClientWithConfig(cfg)

	resp, err := client.CreateChatCompletion(ctx, oai.ChatCompletionRequest{
		Model: model,
		Messages: []oai.ChatCompletionMessage{
			{Role: oai.ChatMessageRoleUser, Content: userInput},
		},
		MaxTokens: 8192,
	})
	if err != nil {
		return "", wrapChatCompletionError(err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("groq: empty choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
