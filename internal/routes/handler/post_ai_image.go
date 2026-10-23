package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/ai"
	"github.com/nikiaprian/tirtayasa/internal/ai/gemini"
	"github.com/nikiaprian/tirtayasa/internal/ai/openai"
	"github.com/nikiaprian/tirtayasa/internal/ai/promptspec"
	"github.com/nikiaprian/tirtayasa/internal/media"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/storage"
)

func imageAIModelName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return openai.ImageModelName()
	case "gemini":
		return gemini.ImageModelName()
	default:
		return ""
	}
}

func imageAIPurposeLabel(forContent bool) string {
	if forContent {
		return "content"
	}
	return "cover"
}

func imageAIProviderForLog(used, requested string) string {
	used = strings.TrimSpace(used)
	if used != "" {
		return used
	}
	return strings.TrimSpace(requested)
}

func imageAILogMessage(purpose, provider, url string) string {
	parts := []string{
		"purpose=" + strings.TrimSpace(purpose),
		"provider=" + strings.TrimSpace(provider),
	}
	if model := imageAIModelName(provider); model != "" {
		parts = append(parts, "model="+model)
	}
	if u := strings.TrimSpace(url); u != "" {
		parts = append(parts, "url="+u)
	}
	return strings.Join(parts, " ")
}

func (h *PostHandler) resolveImagePrompt(title, prompt string, forContent bool) (string, error) {
	p := strings.TrimSpace(prompt)
	if p != "" {
		return strings.ReplaceAll(p, "{{title}}", strings.TrimSpace(title)), nil
	}
	row, err := h.aiSet.GetOrCreate()
	if err != nil {
		return "", err
	}
	var tpl string
	if forContent {
		tpl = strings.TrimSpace(row.GeneratePromptImageToContent)
		if tpl == "" {
			tpl = defaultGenerateImageToContentPrompt
		}
	} else {
		tpl = strings.TrimSpace(row.GeneratePromptImage)
		if tpl == "" {
			tpl = defaultGenerateImagePrompt
		}
	}
	return strings.ReplaceAll(tpl, "{{title}}", strings.TrimSpace(title)), nil
}

func (h *PostHandler) generateAICoverImage(ctx context.Context, title, prompt, provider, slug string, forContent bool) (string, string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "auto"
	}
	keys, err := ai.NewRepoKeysProvider(h.aiSet).Keys(ctx)
	if err != nil {
		return "", "", err
	}
	imagePrompt, err := h.resolveImagePrompt(title, prompt, forContent)
	if err != nil {
		return "", "", err
	}
	openaiKey := strings.TrimSpace(keys.OpenAI) != ""
	geminiKey := strings.TrimSpace(keys.Gemini) != ""

	// Cover images: Groq/Claude cannot generate images directly.
	// "auto" should use whichever image-capable provider key is available.
	providers := make([]string, 0, 2)
	switch provider {
	case "gemini":
		providers = append(providers, "gemini")
	case "openai":
		providers = append(providers, "openai")
	case "groq", "claude", "auto", "":
		if openaiKey {
			providers = append(providers, "openai")
		}
		if geminiKey {
			providers = append(providers, "gemini")
		}
		if len(providers) == 0 {
			// Keep preferred order even when keys are missing so error path is consistent.
			providers = append(providers, "openai", "gemini")
		}
	default:
		if openaiKey {
			providers = append(providers, "openai")
		}
		if geminiKey {
			providers = append(providers, "gemini")
		}
		if len(providers) == 0 {
			providers = append(providers, "openai", "gemini")
		}
	}

	if !openaiKey && !geminiKey {
		return "", "", fmt.Errorf("%w", ai.ErrNoImageProviderKey)
	}
	if len(providers) == 1 && providers[0] == "gemini" && !geminiKey {
		return "", "", fmt.Errorf("%w: gemini", ai.ErrMissingAPIKey)
	}
	var (
		imgRawBase64  string
		genRasterMIME string
		used          string
		lastErr       error
	)
	for _, p := range providers {
		switch p {
		case "openai":
			if strings.TrimSpace(keys.OpenAI) == "" {
				lastErr = fmt.Errorf("%w: openai", ai.ErrMissingAPIKey)
				continue
			}
			out, genMIME, genErr := openai.GenerateImageBase64(ctx, keys.OpenAI, imagePrompt)
			if genErr != nil {
				lastErr = genErr
				continue
			}
			imgRawBase64 = out
			genRasterMIME = genMIME
			used = "openai"
		case "gemini":
			if strings.TrimSpace(keys.Gemini) == "" {
				lastErr = fmt.Errorf("%w: gemini", ai.ErrMissingAPIKey)
				continue
			}
			out, genMIME, genErr := gemini.GenerateImageBase64(ctx, keys.Gemini, imagePrompt)
			if genErr != nil {
				lastErr = genErr
				continue
			}
			imgRawBase64 = out
			genRasterMIME = genMIME
			used = "gemini"
		}
		if strings.TrimSpace(imgRawBase64) != "" {
			break
		}
	}
	if strings.TrimSpace(imgRawBase64) == "" {
		if lastErr != nil {
			return "", "", lastErr
		}
		return "", "", ai.ErrAutoProviderUnavailable
	}
	raw, err := base64.StdEncoding.DecodeString(imgRawBase64)
	if err != nil {
		return "", "", fmt.Errorf("decode generated image: %w", err)
	}
	spec := promptspec.ImageFromPrompt(imagePrompt)
	mime := strings.TrimSpace(genRasterMIME)
	if mime == "" {
		mime = promptspec.RasterMIMEForOutputFormat(spec.OpenAIOutputFormat)
	}
	if spec.StoreWidth > 0 && spec.StoreHeight > 0 {
		resized, rerr := media.ResizeRasterToCoverExact(raw, mime, spec.StoreWidth, spec.StoreHeight)
		if rerr != nil {
			return "", "", fmt.Errorf("resize generated image: %w", rerr)
		}
		raw = resized
		mime = "image/png"
	}
	maxEdge, webpQ := repositories.PostImageStoreLimitsFromDB(h.db)
	enc, err := media.EncodePostUploadWithStoreCap(raw, mime, maxEdge, webpQ)
	if err != nil {
		return "", "", err
	}
	filename, err := postImageFilename(slug, enc.Ext)
	if err != nil {
		return "", "", err
	}
	if store := storage.LoadR2ObjectStore(ctx, h.db, h.cfg); store != nil {
		key := "posts/" + filename
		url, err := store.Put(ctx, key, bytes.NewReader(enc.Data), enc.ContentType)
		if err == nil {
			return strings.TrimSpace(url), used, nil
		}
	}
	if err := savePostImageToDisk(filename, enc.Data); err != nil {
		return "", "", err
	}
	return postImageURLBase + "/" + filename, used, nil
}

func savePostImageToDisk(filename string, data []byte) error {
	if err := os.MkdirAll(postImageDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(postImageDir, filename), data, 0o644)
}
