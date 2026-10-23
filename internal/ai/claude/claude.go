package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const apiURL = "https://api.anthropic.com/v1/messages"

// APIVersion is sent as anthropic-version (Messages API).
const APIVersion = "2023-06-01"

// DefaultModel is used when GenerateContent is called with an empty model name.
const DefaultModel = "claude-3-5-sonnet-20241022"

type messagesRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// GenerateContent calls the Anthropic Messages API and returns assistant text.
func GenerateContent(ctx context.Context, apiKey, model, userInput string) (string, error) {
	if apiKey == "" {
		return "", errors.New("claude: empty API key")
	}
	if userInput == "" {
		return "", errors.New("claude: empty user input")
	}
	if model == "" {
		model = DefaultModel
	}

	var reqBody messagesRequest
	reqBody.Model = model
	reqBody.MaxTokens = 8192
	reqBody.Messages = append(reqBody.Messages, struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{Role: "user", Content: userInput})

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("claude: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("claude: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(apiKey))
	req.Header.Set("anthropic-version", APIVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", fmt.Errorf("claude: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", WrapGenerateError(&httpAPIError{statusCode: resp.StatusCode, body: string(body)})
	}

	var out messagesResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("claude: decode response: %w", err)
	}
	var b strings.Builder
	for _, block := range out.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(block.Text)
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "", errors.New("claude: empty assistant text in response")
	}
	return text, nil
}
