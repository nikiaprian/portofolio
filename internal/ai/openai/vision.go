package openai

import (
	"context"
	"encoding/base64"
	"errors"

	oai "github.com/sashabaranov/go-openai"
)

// GenerateContentWithImage calls OpenAI chat completions with a text prompt and one inline image.
func GenerateContentWithImage(ctx context.Context, apiKey, model, userInput, mime string, imageRaw []byte, searchMode bool) (string, error) {
	if apiKey == "" {
		return "", errors.New("openai: empty API key")
	}
	if userInput == "" {
		return "", errors.New("openai: empty user input")
	}
	if len(imageRaw) == 0 {
		return "", errors.New("openai: empty image data")
	}
	if mime == "" {
		mime = "image/png"
	}
	if model == "" {
		model = DefaultModel
	}

	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imageRaw)
	client := oai.NewClient(apiKey)
	req := oai.ChatCompletionRequest{
		Model: model,
		Messages: []oai.ChatCompletionMessage{
			{
				Role: oai.ChatMessageRoleUser,
				MultiContent: []oai.ChatMessagePart{
					{Type: oai.ChatMessagePartTypeText, Text: userInput},
					{
						Type: oai.ChatMessagePartTypeImageURL,
						ImageURL: &oai.ChatMessageImageURL{
							URL: dataURL,
						},
					},
				},
			},
		},
		MaxTokens: 8192,
	}
	if searchMode {
		req.Tools = []oai.Tool{
			{Type: "web_search"},
		}
	}
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", wrapChatCompletionError(err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("openai: empty choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}
