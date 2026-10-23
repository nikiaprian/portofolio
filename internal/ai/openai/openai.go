package openai

import (
	"context"
	"errors"
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/ai/promptspec"
	oai "github.com/sashabaranov/go-openai"
)

// DefaultModel is used when GenerateContent is called with an empty model name.
// Use a "full" model by default so long-form CMS drafts are comparable to Gemini output.
const DefaultModel = oai.GPT4o

// ImageModelName is the OpenAI image generation model used by GenerateImageBase64.
func ImageModelName() string {
	return oai.CreateImageModelGptImage1
}

// GenerateContent calls the OpenAI Chat Completions API and returns the assistant message text.
func GenerateContent(ctx context.Context, apiKey, model, userInput string) (string, error) {
	if apiKey == "" {
		return "", errors.New("openai: empty API key")
	}
	if userInput == "" {
		return "", errors.New("openai: empty user input")
	}
	if model == "" {
		model = DefaultModel
	}

	client := oai.NewClient(apiKey)
	resp, err := client.CreateChatCompletion(ctx, oai.ChatCompletionRequest{
		Model: model,
		Messages: []oai.ChatCompletionMessage{
			{Role: oai.ChatMessageRoleUser, Content: userInput},
		},
		// Default API cap is often too low for long HTML drafts; continuations also need headroom.
		MaxTokens: 8192,
	})
	if err != nil {
		return "", wrapChatCompletionError(err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("openai: empty choices in response")
	}
	return resp.Choices[0].Message.Content, nil
}

func mapOpenAIImageQuality(q string) string {
	switch strings.ToLower(strings.TrimSpace(q)) {
	case "low":
		return oai.CreateImageQualityLow
	case "high":
		return oai.CreateImageQualityHigh
	default:
		return oai.CreateImageQualityMedium
	}
}

func mapOpenAIOutputFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "png":
		return oai.CreateImageOutputFormatPNG
	case "jpeg", "jpg":
		return oai.CreateImageOutputFormatJPEG
	default:
		return oai.CreateImageOutputFormatWEBP
	}
}

func mimeForOpenAIOutputFormat(apiFormat string) string {
	switch apiFormat {
	case oai.CreateImageOutputFormatPNG:
		return "image/png"
	case oai.CreateImageOutputFormatJPEG:
		return "image/jpeg"
	default:
		return "image/webp"
	}
}

// GenerateImageBase64 calls OpenAI image generation and returns base64 payload and
// the corresponding image MIME type for decoding (from requested output format).
func GenerateImageBase64(ctx context.Context, apiKey, prompt string) (string, string, error) {
	if apiKey == "" {
		return "", "", errors.New("openai: empty API key")
	}
	if prompt == "" {
		return "", "", errors.New("openai: empty image prompt")
	}
	spec := promptspec.ImageFromPrompt(prompt)
	size := spec.OpenAIImageSize
	switch size {
	case oai.CreateImageSize1024x1024, oai.CreateImageSize1024x1536, oai.CreateImageSize1536x1024:
	default:
		size = promptspec.DefaultOpenAIImageSize
	}
	outFmt := mapOpenAIOutputFormat(spec.OpenAIOutputFormat)
	mime := mimeForOpenAIOutputFormat(outFmt)
	client := oai.NewClient(apiKey)
	resp, err := client.CreateImage(ctx, oai.ImageRequest{
		Model:        ImageModelName(),
		Prompt:       prompt,
		Size:         size,
		Quality:      mapOpenAIImageQuality(spec.OpenAIQuality),
		OutputFormat: outFmt,
		N:            1,
	})
	if err != nil {
		return "", "", wrapOpenAIAPIError(err)
	}
	if len(resp.Data) == 0 || resp.Data[0].B64JSON == "" {
		return "", "", errors.New("openai: empty image response")
	}
	return resp.Data[0].B64JSON, mime, nil
}
