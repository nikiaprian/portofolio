package scraper

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nikiaprian/tirtayasa/internal/ai"
	"github.com/nikiaprian/tirtayasa/internal/ai/gemini"
	"github.com/nikiaprian/tirtayasa/internal/ai/openai"
	"github.com/nikiaprian/tirtayasa/internal/ai/promptspec"
	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/media"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/storage"
	"gorm.io/gorm"
)

func scraperImageAIModelName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return openai.ImageModelName()
	case "gemini":
		return gemini.ImageModelName()
	default:
		return ""
	}
}

func scraperImageAILogMessage(purpose, provider, url string) string {
	parts := []string{
		"purpose=" + strings.TrimSpace(purpose),
		"provider=" + strings.TrimSpace(provider),
	}
	if model := scraperImageAIModelName(provider); model != "" {
		parts = append(parts, "model="+model)
	}
	if u := strings.TrimSpace(url); u != "" {
		parts = append(parts, "url="+u)
	}
	return strings.Join(parts, " ")
}

const scraperDefaultImagePrompt = `Buat gambar cover artikel berkualitas sedang dengan rasio horizontal (landscape) yang umum untuk artikel, utamakan 16:9, berdasarkan judul: "{{title}}".

Patokan utama:
- Seluruh konsep visual WAJIB mengacu pada makna judul artikel/post.
- Pilih satu subjek visual utama yang paling mewakili judul.
- Jika judul spesifik (orang, profesi, tempat, peristiwa, produk), tampilkan elemen yang relevan dan mudah dipahami.

Gaya dan kualitas:
- Gaya editorial modern untuk berita/blog.
- Komposisi bersih, fokus jelas, pencahayaan realistis/sinematik.
- Kontras kuat, detail tajam, cocok untuk cover artikel/thumbnail website.
- Subjek utama ditempatkan aman di area tengah agar tetap bagus saat di-crop.

Batasan:
- Tanpa teks, logo, watermark, elemen UI, atau brand mark.
- Hindari blur berlebihan, noise, anatomi rusak, dan latar terlalu ramai.
`

const scraperDefaultImageToContentPrompt = `Buat ilustrasi artikel berkualitas sedang untuk disisipkan di tengah isi tulisan (bukan thumbnail cover), rasio landscape atau netral (utamakan 3:2 atau 16:9), berdasarkan judul: "{{title}}".

Patokan utama:
- Visual mendukung atau menjelaskan gagasan utama topik artikel; cocok dibaca bersama paragraf di sekitarnya.
- Hindari komposisi yang hanya optimal untuk crop kecil seperti thumbnail feed.

Gaya dan kualitas:
- Gaya editorial modern; fokus jelas, pencahayaan realistis/sinematik.
- Satu ide visual utama; latar tidak mengganggu.

Batasan:
- Tanpa teks, logo, watermark, elemen UI, atau brand mark.
- Hindari blur berlebihan, noise, anatomi rusak, dan latar terlalu ramai.
`

func resolveScraperAIImagePrompt(db *gorm.DB, title string, forContent bool) (string, error) {
	title = strings.TrimSpace(title)
	var row entity.AISetting
	if err := db.First(&row).Error; err != nil {
		if forContent {
			return strings.ReplaceAll(scraperDefaultImageToContentPrompt, "{{title}}", title), nil
		}
		return strings.ReplaceAll(scraperDefaultImagePrompt, "{{title}}", title), nil
	}
	var tpl string
	if forContent {
		tpl = strings.TrimSpace(row.GeneratePromptImageToContent)
		if tpl == "" {
			tpl = scraperDefaultImageToContentPrompt
		}
	} else {
		tpl = strings.TrimSpace(row.GeneratePromptImage)
		if tpl == "" {
			tpl = scraperDefaultImagePrompt
		}
	}
	return strings.ReplaceAll(tpl, "{{title}}", title), nil
}

func scraperPostImageFilename(slug, ext string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug != "" && len(slug) <= scrapedSlugMaxLen && scrapedImageSlugRe.MatchString(slug) {
		return fmt.Sprintf("%s-%d%s", slug, time.Now().Unix(), ext), nil
	}
	return newScrapedImageFilename(ext)
}

// generateAIPostCoverForScraper creates an image via OpenAI or Gemini (same rules as admin bulk/post generate-image)
// and stores it like other post uploads. forContent selects the in-body prompt template when the custom prompt path is unused.
// Returns public URL and provider used, or empty values on failure.
func generateAIPostCoverForScraper(
	ctx context.Context,
	db *gorm.DB,
	store storage.ObjectStore,
	setting *runtimeSetting,
	title, slug string,
	forContent bool,
) (string, string, error) {
	if ctx == nil || db == nil {
		return "", "", fmt.Errorf("missing context or db")
	}
	if setting == nil {
		return "", "", fmt.Errorf("missing scraper runtime setting")
	}
	kp := ai.NewRepoKeysProvider(repositories.NewAISettingRepository(db))
	keys, err := kp.Keys(ctx)
	if err != nil {
		return "", "", err
	}
	imagePrompt, err := resolveScraperAIImagePrompt(db, title, forContent)
	if err != nil {
		return "", "", err
	}
	provider := strings.ToLower(strings.TrimSpace(setting.ScraperAIProvider))
	if provider == "" {
		provider = "auto"
	}
	openaiKey := strings.TrimSpace(keys.OpenAI) != ""
	geminiKey := strings.TrimSpace(keys.Gemini) != ""

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
		imgRawBase64 string
		imgInputMIME string
		used         string
		lastErr      error
	)
	for _, p := range providers {
		switch p {
		case "openai":
			if strings.TrimSpace(keys.OpenAI) == "" {
				lastErr = fmt.Errorf("%w: openai", ai.ErrMissingAPIKey)
				continue
			}
			out, mime, genErr := openai.GenerateImageBase64(ctx, keys.OpenAI, imagePrompt)
			if genErr != nil {
				lastErr = genErr
				continue
			}
			imgRawBase64 = out
			imgInputMIME = mime
			used = "openai"
		case "gemini":
			if strings.TrimSpace(keys.Gemini) == "" {
				lastErr = fmt.Errorf("%w: gemini", ai.ErrMissingAPIKey)
				continue
			}
			out, mime, genErr := gemini.GenerateImageBase64(ctx, keys.Gemini, imagePrompt)
			if genErr != nil {
				lastErr = genErr
				continue
			}
			imgRawBase64 = out
			imgInputMIME = mime
			used = "gemini"
		}
		if strings.TrimSpace(imgRawBase64) != "" {
			break
		}
	}
	if strings.TrimSpace(imgRawBase64) == "" {
		if lastErr != nil {
			return "", used, lastErr
		}
		return "", used, ai.ErrAutoProviderUnavailable
	}
	raw, err := base64.StdEncoding.DecodeString(imgRawBase64)
	if err != nil {
		return "", used, fmt.Errorf("decode generated image: %w", err)
	}
	spec := promptspec.ImageFromPrompt(imagePrompt)
	inputMIME := strings.TrimSpace(imgInputMIME)
	if inputMIME == "" {
		inputMIME = promptspec.RasterMIMEForOutputFormat(spec.OpenAIOutputFormat)
	}
	if spec.StoreWidth > 0 && spec.StoreHeight > 0 {
		resized, rerr := media.ResizeRasterToCoverExact(raw, inputMIME, spec.StoreWidth, spec.StoreHeight)
		if rerr != nil {
			return "", used, fmt.Errorf("resize generated image: %w", rerr)
		}
		raw = resized
		inputMIME = "image/png"
	}
	maxEdge, webpQ := repositories.PostImageStoreLimitsFromDB(db)
	enc, err := media.EncodePostUploadWithStoreCap(raw, inputMIME, maxEdge, webpQ)
	if err != nil {
		return "", used, err
	}
	filename, err := scraperPostImageFilename(slug, enc.Ext)
	if err != nil {
		return "", used, err
	}
	if store != nil {
		key := "posts/" + filename
		url, putErr := store.Put(ctx, key, bytes.NewReader(enc.Data), enc.ContentType)
		if putErr == nil {
			return strings.TrimSpace(url), used, nil
		}
		ScrapeLogf("warn", "", "", "[scraper] AI cover R2 upload failed, using local disk: %v", putErr)
	}
	if err := os.MkdirAll(scrapedImageDir, 0o755); err != nil {
		return "", used, err
	}
	targetPath := filepath.Join(scrapedImageDir, filename)
	if err := os.WriteFile(targetPath, enc.Data, 0o644); err != nil {
		return "", used, err
	}
	return scrapedImageURLBase + "/" + filename, used, nil
}
