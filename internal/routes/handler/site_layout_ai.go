package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/ai"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
)

var siteLayoutFragmentKeys = []string{
	"header",
	"footer",
	"home_body",
	"article_body",
	"sidebar",
	"rss",
	"author_profile",
	"tag",
	"kategori",
	"search_page",
}

var siteLayoutFragmentRefFiles = map[string]string{
	"header":         "public/templates/layouts/client/header.html",
	"footer":         "public/templates/layouts/client/footer.html",
	"home_body":      "public/templates/pages/client/index.html",
	"article_body":   "public/templates/pages/client/post.html",
	"sidebar":        "public/templates/partials/client/blog_sidebar.html",
	"rss":            "public/templates/pages/client/rss_feeds.html",
	"author_profile": "public/templates/pages/client/author.html",
	"tag":            "public/templates/pages/client/tag.html",
	"kategori":       "public/templates/pages/client/category.html",
	"search_page":    "public/templates/pages/client/posts.html",
}

const siteLayoutDefaultCurrentPrompt = `Act as a Senior Frontend Engineer, UI/UX Engineer, and Software Architect.

Recreate the provided website screenshot as a Go HTML template fragment for a CMS custom layout.

Requirements:
- Output a single Go html/template fragment (use {{ }} for dynamic values like the reference template).
- Prefix every CSS class with TEMPLATE_NAME (e.g. template-brand-header).
- Prefix JS identifiers with the camelCase form of TEMPLATE_NAME.
- No inline CSS or JavaScript in the HTML fragment.
- Mobile-first, responsive, semantic HTML.
- Match spacing, typography, colors, alignment, and visual hierarchy from the screenshot.
- Reuse the same template variables and partial patterns shown in the reference template.
- Return ONLY the template code for the requested fragment — no markdown fences, no analysis.`

var siteLayoutAIOutputFenceRe = regexp.MustCompile("(?is)```(?:html|go|template)?\\s*([\\s\\S]*?)```")

// GenerateFragment uses vision AI to build one layout fragment from a reference screenshot.
func (h *SiteLayoutHandler) GenerateFragment(c *gin.Context) {
	if h.ai == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generator AI belum dikonfigurasi"})
		return
	}

	var req dto.GenerateSiteLayoutFragmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}

	fragment := normalizeSiteLayoutFragment(req.Fragment)
	if fragment == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fragment tidak valid"})
		return
	}

	layoutName := strings.TrimSpace(req.LayoutName)
	if layoutName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama layout wajib diisi sebelum generate"})
		return
	}

	imageURL, err := h.resolveSiteLayoutImageURL(strings.TrimSpace(req.ImageURL), req.ImageRefs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	baseURL := publicCanonicalSiteBaseURL(c)
	imageRaw, mime, err := loadSiteLayoutRefImage(imageURL, baseURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	refContent, err := readSiteLayoutReferenceTemplate(fragment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat template referensi"})
		return
	}

	currentPrompt := strings.TrimSpace(req.CurrentPrompt)
	if currentPrompt == "" {
		currentPrompt = siteLayoutDefaultCurrentPrompt
	}

	templateName := siteLayoutTemplatePrefix(layoutName)
	prompt := buildSiteLayoutFragmentPrompt(fragment, templateName, currentPrompt, refContent)

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "auto"
	}

	ctx := c.Request.Context()
	var text string
	var usedProv string
	switch provider {
	case "auto":
		var used ai.Provider
		text, used, err = h.ai.GenerateBlogContentWithImageAndProvider(ctx, prompt, mime, imageRaw)
		if err != nil {
			siteLayoutRespondAIError(c, err, provider)
			return
		}
		usedProv = string(used)
	default:
		text, err = h.ai.GenerateWithImage(ctx, ai.Provider(provider), prompt, mime, imageRaw)
		if err != nil {
			siteLayoutRespondAIError(c, err, provider)
			return
		}
		usedProv = provider
	}

	content := extractSiteLayoutAIOutput(text)
	if content == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI mengembalikan fragment kosong"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fragment":  fragment,
		"content":   content,
		"provider":  usedProv,
		"image_url": imageURL,
	})
}

// ClassifyImageRefs maps each uploaded reference image to a layout fragment type.
func (h *SiteLayoutHandler) ClassifyImageRefs(c *gin.Context) {
	if h.ai == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generator AI belum dikonfigurasi"})
		return
	}

	var req dto.ClassifySiteLayoutImageRefsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "auto"
	}

	mapping := make(map[string]string, len(req.ImageRefs))
	ctx := c.Request.Context()
	baseURL := publicCanonicalSiteBaseURL(c)

	for _, rawURL := range req.ImageRefs {
		imageURL := strings.TrimSpace(rawURL)
		if imageURL == "" {
			continue
		}
		imageRaw, mime, err := loadSiteLayoutRefImage(imageURL, baseURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Gagal memuat gambar %s: %v", imageURL, err)})
			return
		}

		prompt := `Look at this website screenshot. Which ONE layout fragment type does it best represent?

Options (reply with ONLY the key, nothing else):
header
footer
home_body
article_body
sidebar
rss
author_profile
tag
kategori
search_page
unknown`

		var text string
		if provider == "auto" {
			text, _, err = h.ai.GenerateBlogContentWithImageAndProvider(ctx, prompt, mime, imageRaw)
		} else {
			text, err = h.ai.GenerateWithImage(ctx, ai.Provider(provider), prompt, mime, imageRaw)
		}
		if err != nil {
			siteLayoutRespondAIError(c, err, provider)
			return
		}

		key := normalizeSiteLayoutFragment(strings.TrimSpace(strings.ToLower(text)))
		if key == "" {
			key = "unknown"
		}
		mapping[imageURL] = key
	}

	c.JSON(http.StatusOK, gin.H{"mapping": mapping})
}

func (h *SiteLayoutHandler) resolveSiteLayoutImageURL(imageURL string, imageRefs []string) (string, error) {
	if imageURL != "" {
		return imageURL, nil
	}

	refs := make([]string, 0, len(imageRefs))
	for _, u := range imageRefs {
		if s := strings.TrimSpace(u); s != "" {
			refs = append(refs, s)
		}
	}
	if len(refs) == 0 {
		return "", errors.New("Unggah minimal satu gambar referensi sebelum generate")
	}
	return refs[0], nil
}

func normalizeSiteLayoutFragment(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	for _, key := range siteLayoutFragmentKeys {
		if s == key {
			return key
		}
	}
	return ""
}

func siteLayoutTemplatePrefix(layoutName string) string {
	s := strings.ToLower(strings.TrimSpace(layoutName))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "layout"
	}
	return "template-" + slug
}

func buildSiteLayoutFragmentPrompt(fragment, templateName, currentPrompt, refContent string) string {
	var b strings.Builder
	b.WriteString(currentPrompt)
	b.WriteString("\n\n=========================================\n")
	b.WriteString("RUNTIME CONTEXT\n")
	b.WriteString("=========================================\n\n")
	b.WriteString("TEMPLATE_NAME = \"")
	b.WriteString(templateName)
	b.WriteString("\"\n\n")
	b.WriteString("FRAGMENT TO GENERATE = ")
	b.WriteString(fragment)
	b.WriteString("\n\n")
	b.WriteString("REFERENCE TEMPLATE (existing CMS fragment — mirror variables, partials, and conventions):\n\n")
	b.WriteString(refContent)
	b.WriteString("\n\n")
	b.WriteString("OUTPUT RULES:\n")
	b.WriteString("- Return ONLY the Go template HTML for fragment \"")
	b.WriteString(fragment)
	b.WriteString("\".\n")
	b.WriteString("- Do not wrap output in markdown code fences.\n")
	b.WriteString("- Keep dynamic data as Go template expressions like the reference.\n")
	b.WriteString("- Put page-specific CSS/JS in separate comments at the end only if unavoidable; prefer class prefixes for CSS in css_data later.\n")
	return b.String()
}

func readSiteLayoutReferenceTemplate(fragment string) (string, error) {
	path, ok := siteLayoutFragmentRefFiles[fragment]
	if !ok {
		return "", fmt.Errorf("unknown fragment %q", fragment)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	const maxRef = 120_000
	if len(raw) > maxRef {
		raw = raw[:maxRef]
	}
	return string(raw), nil
}

func loadSiteLayoutRefImage(imageURL, baseURL string) ([]byte, string, error) {
	u := strings.TrimSpace(imageURL)
	if u == "" {
		return nil, "", errors.New("URL gambar referensi kosong")
	}

	if localPath := siteLayoutLocalDiskPath(u); localPath != "" {
		if raw, err := os.ReadFile(localPath); err == nil && len(raw) > 0 {
			return raw, http.DetectContentType(raw), nil
		}
	}

	fetchURL := siteLayoutAbsoluteImageURL(u, baseURL)
	if fetchURL == "" {
		return nil, "", errors.New("URL gambar referensi tidak valid")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fetchURL)
	if err != nil {
		return nil, "", fmt.Errorf("gagal mengunduh gambar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("gagal mengunduh gambar: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, layoutRefImageMaxBytes))
	if err != nil {
		return nil, "", fmt.Errorf("gagal membaca gambar: %w", err)
	}
	if len(raw) == 0 {
		return nil, "", errors.New("gambar referensi kosong")
	}
	return raw, http.DetectContentType(raw), nil
}

func siteLayoutLocalDiskPath(imageURL string) string {
	u := strings.TrimSpace(imageURL)
	switch {
	case strings.HasPrefix(u, "/static/"):
		return filepath.Join("public", strings.TrimPrefix(u, "/"))
	case strings.HasPrefix(u, "static/"):
		return filepath.Join("public", u)
	default:
		return ""
	}
}

func siteLayoutAbsoluteImageURL(imageURL, baseURL string) string {
	u := strings.TrimSpace(imageURL)
	if u == "" {
		return ""
	}
	low := strings.ToLower(u)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return u
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return u
	}
	if strings.HasPrefix(u, "/") {
		return base + u
	}
	return base + "/" + u
}

func extractSiteLayoutAIOutput(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}
	if m := siteLayoutAIOutputFenceRe.FindStringSubmatch(s); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return s
}

func siteLayoutRespondAIError(c *gin.Context, err error, _ string) {
	if code, prov, ok := aiQuotaHTTPMeta(err); ok {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":    err.Error(),
			"code":     code,
			"provider": prov,
		})
		return
	}
	if errors.Is(err, ai.ErrAutoProviderUnavailable) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": `Penyedia AI "auto" belum dikonfigurasi untuk vision (Gemini atau OpenAI). Tambahkan API key di Admin -> Pengaturan AI.`,
		})
		return
	}
	if errors.Is(err, ai.ErrMissingAPIKey) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI belum dikonfigurasi. Tambahkan minimal satu key penyedia di Admin -> Pengaturan AI.",
		})
		return
	}
	if errors.Is(err, ai.ErrUnsupportedProvider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Penyedia tidak mendukung analisis gambar. Gunakan Gemini atau OpenAI."})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}
