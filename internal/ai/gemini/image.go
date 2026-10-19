package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Bebii-Digital/bebii-blog/internal/ai/promptspec"
)

// Official Gemini image model (Google AI Studio / Gemini API).
const geminiImageModel = "gemini-3-pro-image-preview"

// ImageModelName is the Gemini image generation model used by GenerateImageBase64.
func ImageModelName() string {
	return geminiImageModel
}

func geminiAspectRatioAllowed(s string) bool {
	switch strings.TrimSpace(s) {
	case "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9":
		return true
	default:
		return false
	}
}

// GenerateImageBase64 generates an image with Gemini and returns base64 image bytes
// and the response MIME type (for decoding), e.g. image/png or image/jpeg.
func GenerateImageBase64(ctx context.Context, apiKey, prompt string) (string, string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", "", errors.New("gemini: empty API key")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", "", errors.New("gemini: empty image prompt")
	}
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" +
		ImageModelName() +
		":generateContent?key=" +
		url.QueryEscape(strings.TrimSpace(apiKey))
	spec := promptspec.ImageFromPrompt(prompt)
	aspect := spec.GeminiAspectRatio
	if !geminiAspectRatioAllowed(aspect) {
		aspect = promptspec.DefaultGeminiAspectRatio
	}
	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"IMAGE"},
			"imageConfig": map[string]any{
				"aspectRatio": aspect,
			},
		},
	}
	rawBody, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", WrapGenerateError(err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", "", WrapGenerateError(err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", "", ErrQuotaOrRateLimit
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", WrapGenerateError(fmt.Errorf("gemini image http %d: %s", resp.StatusCode, strings.TrimSpace(string(payload))))
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", "", WrapGenerateError(err)
	}
	candidates, _ := decoded["candidates"].([]any)
	for _, c := range candidates {
		cm, _ := c.(map[string]any)
		content, _ := cm["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, p := range parts {
			pm, _ := p.(map[string]any)
			inlineData, _ := pm["inlineData"].(map[string]any)
			if inlineData == nil {
				continue
			}
			if b64, ok := inlineData["data"].(string); ok && strings.TrimSpace(b64) != "" {
				b64 = strings.TrimSpace(b64)
				mime := strings.TrimSpace(fmt.Sprint(inlineData["mimeType"]))
				if mime == "" {
					if raw, err := base64.StdEncoding.DecodeString(b64); err == nil && len(raw) > 0 {
						mime = http.DetectContentType(raw)
					}
					if mime == "" || mime == "application/octet-stream" {
						mime = promptspec.RasterMIMEForOutputFormat(spec.OpenAIOutputFormat)
					}
				}
				return b64, mime, nil
			}
		}
	}
	return "", "", WrapGenerateError(errors.New("gemini image: empty response"))
}
