package opencode

import (
	"context"
	"errors"
	"strings"

	oai "github.com/sashabaranov/go-openai"
)

// APIBaseURL is OpenCode Zen's OpenAI-compatible endpoint.
const APIBaseURL = "https://opencode.ai/zen/v1"

// DefaultModel is used when GenerateContent is called with an empty model name.
const DefaultModel = "mimo-v2.5-free"

// GenerateContent calls OpenCode Zen's chat completions API and returns the assistant message text.
func GenerateContent(ctx context.Context, apiKey, model, userInput string, searchMode bool) (string, error) {
	if apiKey == "" {
		return "", errors.New("opencode: empty API key")
	}
	if userInput == "" {
		return "", errors.New("opencode: empty user input")
	}
	model = strings.TrimSpace(model)
	reqContent := userInput

	if searchMode {
		reqContent = userInput + "\n\n[Note: Real-time web search mode is ENABLED. Please provide accurate, real-time, and up-to-date information for current events, rankings, and statistics.]"
	}

	if model == "" {
		model = DefaultModel
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
	if err != nil {
		return "", wrapChatCompletionError(err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("opencode: empty choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
