package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	appconfig "github.com/nikiaprian/tirtayasa/config"
	"github.com/nikiaprian/tirtayasa/internal/ai"
	aiclaud "github.com/nikiaprian/tirtayasa/internal/ai/claude"
	aigemini "github.com/nikiaprian/tirtayasa/internal/ai/gemini"
	aigroq "github.com/nikiaprian/tirtayasa/internal/ai/groq"
	aiopenai "github.com/nikiaprian/tirtayasa/internal/ai/openai"
	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/autohref"
	"github.com/nikiaprian/tirtayasa/internal/helper/feedtitles"
	"github.com/nikiaprian/tirtayasa/internal/helper/posthtml"
	"github.com/nikiaprian/tirtayasa/internal/helper/postmeta"
	"github.com/nikiaprian/tirtayasa/internal/media"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
	"github.com/nikiaprian/tirtayasa/internal/scraper"
	"github.com/nikiaprian/tirtayasa/internal/services"
	"github.com/nikiaprian/tirtayasa/internal/social"
	"github.com/nikiaprian/tirtayasa/internal/storage"
	"gorm.io/gorm"
)

// bulkMaxArticleTitleLines caps Article titles lines for bulk scrape, regenerate, and generate.
const bulkMaxArticleTitleLines = 1000

// bulkRegenerateLinesPerAPICall stays under typical ~8k completion-token limits so the model
// does not truncate the tail (which previously surfaced as blank title lines).
const bulkRegenerateLinesPerAPICall = 48

func requesterFromContext(c *gin.Context) (uint, string) {
	uidAny, ok := c.Get("userID")
	if !ok {
		return 0, ""
	}
	uid, _ := uidAny.(uint)
	roleAny, _ := c.Get("userRole")
	role, _ := roleAny.(string)
	return uid, strings.ToLower(strings.TrimSpace(role))
}

const bulkPostLogMsgMaxRunes = 8000
const manualPostLogMsgMaxRunes = 8000

func truncateBulkPostLogRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}

func normalizeBulkPostProviderCol(s string) string {
	return truncateBulkPostLogRunes(strings.ToLower(strings.TrimSpace(s)), 16)
}

func normalizeManualPostProviderCol(s string) string {
	return truncateBulkPostLogRunes(strings.ToLower(strings.TrimSpace(s)), 16)
}

func (h *PostHandler) persistBulkPostLog(c *gin.Context, level, event, message string, categoryID *uint, postID *uint, title string, providerUsed string) {
	if h.db == nil || c == nil {
		return
	}
	uid, _ := requesterFromContext(c)
	if uid == 0 {
		return
	}
	row := entity.BulkPostLog{
		UserID:       uid,
		Level:        truncateBulkPostLogRunes(strings.TrimSpace(level), 16),
		Event:        truncateBulkPostLogRunes(strings.TrimSpace(event), 32),
		Message:      truncateBulkPostLogRunes(strings.TrimSpace(message), bulkPostLogMsgMaxRunes),
		CategoryID:   categoryID,
		PostID:       postID,
		Title:        truncateBulkPostLogRunes(strings.TrimSpace(title), 220),
		ProviderUsed: normalizeBulkPostProviderCol(providerUsed),
	}
	if row.Level == "" {
		row.Level = "info"
	}
	_ = h.db.Select("UserID", "Level", "Event", "Message", "CategoryID", "PostID", "Title", "ProviderUsed").Create(&row).Error
}

func (h *PostHandler) persistManualPostAILog(c *gin.Context, level, event, message string, postID *uint, title, providerInput, providerUsed string) {
	if h.db == nil || c == nil {
		return
	}
	uid, _ := requesterFromContext(c)
	if uid == 0 {
		return
	}
	row := entity.ManualPostAILog{
		UserID:        uid,
		Level:         truncateBulkPostLogRunes(strings.TrimSpace(level), 16),
		Event:         truncateBulkPostLogRunes(strings.TrimSpace(event), 32),
		Message:       truncateBulkPostLogRunes(strings.TrimSpace(message), manualPostLogMsgMaxRunes),
		PostID:        postID,
		Title:         truncateBulkPostLogRunes(strings.TrimSpace(title), 220),
		ProviderInput: normalizeManualPostProviderCol(providerInput),
		ProviderUsed:  normalizeManualPostProviderCol(providerUsed),
	}
	if row.Level == "" {
		row.Level = "info"
	}
	_ = h.db.Select("UserID", "Level", "Event", "Message", "PostID", "Title", "ProviderInput", "ProviderUsed").Create(&row).Error
}

func respondNotFound(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Data tidak ditemukan"})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Data tidak ditemukan"})
}

const blogDraftSystemPrompt = `You write article body HTML for a blog CMS. The post title is stored in a separate field — never use <h1>. Reply with an HTML fragment only (no markdown fences, no <!DOCTYPE> or <html> wrapper).
Treat the request as ordinary editorial/blog writing. If the topic is sensitive but allowed, write a neutral, informational article with appropriate caveats instead of refusing. Do not provide instructions for illegal activity, evading safety systems, or causing harm.
Start with one or more opening <p> paragraphs (lead), then use <h2>, <h3>, <ul>, <ol>, <li>, <strong>, <em>, and <blockquote> where appropriate.
Keep markup simple and valid.

`

const blogDraftMetaDescriptionSystemPrompt = `You write SEO meta descriptions for web articles. Follow the user's instructions below.
Reply with plain text only: a single line, no quotes, no markdown, no HTML, no label prefix (do not output a line like "Meta description:").

`

const blogDraftMetaKeywordsSystemPrompt = `You write SEO meta keywords for web articles. Follow the user's instructions below.
Reply with plain text only: comma-separated keywords (roughly 5–12 items), no quotes, no numbering, no Markdown, no label prefix.

`

const defaultGeneratePromptMetaDescription = `Ringkas meta description (sekitar 150-160 karakter) yang relevan dengan judul.`

const defaultGeneratePromptMetaKeywords = `Gunakan kata kunci utama secara natural di judul dan isi; hindari keyword stuffing. Jika perlu, sebutkan sinonim atau istilah terkait yang relevan bagi pembaca.`

// blogDraftContinuationSystemPrompt is used only for follow-up API calls (same rules as the first draft: no <h1>).
const blogDraftContinuationSystemPrompt = `You continue ONE existing blog article body for a CMS. The title is stored separately — never use <h1>. Reply with an HTML fragment only (no markdown fences, no <!DOCTYPE> or <html> wrapper).
Treat the request as ordinary editorial/blog writing. If the topic is sensitive but allowed, continue with neutral, informational sections and appropriate caveats instead of refusing.
Never restart the article, never duplicate the introduction, and do not add a second closing / "kesimpulan" / summary section if the excerpt already ends with a conclusion — instead add new middle sections (<h2>, <h3>, <p>, lists, blockquotes) that fit before any existing wrap-up.
Keep markup simple and valid.

`

const bulkTitlesPromptPrefix = `You suggest short blog article titles. Reply with plain text only: one title per line, no numbering, no bullets, no quotes, no extra commentary.

`

const bulkRegenerateTitlesPromptPrefix = `You rephrase or improve existing blog article titles while keeping the SAME line count and order as the input.
Rules:
- Reply with plain text only: exactly one output line per input line (same number of lines).
- Do not add line numbers, bullets, or quotes around titles.
- For a blank input line, output a blank line.
- Each output line must be a single concise article title (keep similar length to the input line unless it was too long).
- Do not merge multiple input lines into one output line.

`

const bulkGenerateJSONSystemPrompt = `You generate blog post content for a CMS.

Return ONLY valid JSON (no markdown fences, no backticks, no extra commentary). The JSON MUST have exactly these keys:
- content: string (HTML fragment only; never use <h1>; no <!DOCTYPE>, no <html> wrapper)
- meta_description: string (150–160 chars recommended; plain text)
- meta_keywords: string (comma-separated keywords; plain text)

Rules:
- content must start with one or more <p> paragraphs, then use <h2>, <h3>, <ul>, <ol>, <li>, <strong>, <em>, <blockquote> where appropriate.
- Keep HTML valid and simple.
- meta_keywords should be concise, relevant to the title and content, and separated by commas.

`

const defaultGeneratePromptContent = `Buatkan artikel lengkap dan teroptimasi SEO tentang {{title}} yang unik, informatif, dan mudah dipahami. Artikel harus memenuhi kriteria berikut:
1. Panjang: Minimal 1000 kata.
2. Struktur: Harus memiliki struktur yang jelas dengan penggunaan subjudul yang efektif (H1, H2, H3) untuk memudahkan pembacaan dan meningkatkan SEO. Setiap bagian harus secara langsung menangani aspek yang berbeda dari {{title}} sesuai dengan User Intent.
Kamu bisa menempatkan keyword pada tengah parapraph pertama atau pada bagian akhir paragraph pertama.
Jadikan artikel ini agar masuk google snippet dan sekaligus discover. Untuk setiap subheading wajib memiliki satu paragraph pembuka, misalkan ada bullet, kamu bisa buat dulu satu paragraph pembuka lalu masuk pada point tersebut.
Jadikan artikel ini beda dengan yang sudah ada di internet, kamu bisa memulai subheading dengan menjelaskan topik atau tema artikel, masuk pada solusi dan terakhir paragraph penutup.
Agar artikel tidak terkesan sama dari sumber yang ada di internet kamu dapat memasukan subheading lainnya, seperti perbandingan, atau jika ada artikel listicle kamu bisa jelaskan satu persatu.
Gunakan Bahasa sederhana, kamu bisa pakai subject atau tidak. Kalau pakai subject dapat gunakan kalian atau kamu jangan anda.
Untuk penggunaan table hanya opsional, jika dibutuhkan bisa ditambahkan.
Untuk paragraph pembuka minimal 3 paragraph, dan jangan ada kalimat seperti pada artikel ini, topik yang banyak dicari, pada pembahasan ini, Kamu buat senatural mungkin.
Prioritas utama adalah panjang artikel. Jangan berhenti menulis sebelum minimal 2500 kata tercapai, walaupun struktur sudah terlihat lengkap. Perluas setiap subheading dengan penjelasan, contoh, perbandingan, dan elaborasi tambaha
Untuk FAQ bisa ditaruh setelah paragraph penutup untuk FAQ sendiri subheading 2 dan pertanyaan subheading 3.
Setiap paragraph jangan terlalu panjang dan jangan ada icon pada artikel, selain itu jangan ada garis untuk setiap subheading.
Untuk keyword turunan kamu dapat mencari di hasil pencarian google di bawah kolom, ambil 3 atau 4 keyword saja.
Jika ada cara atau tutorial maka kamu bisa gunakan bullet agar masuk snippet, selain itu apabila terdapat seperti kata portal resmi, maka kamu bisa tambahkan linknya, jadi lebih spesifik.
Jangan gunakan kata alur, karena ini menurut saya sangat kaku, kamu dapat gunakan cara atau lainnya.
Jangan pakai <h1> — judul artikel sudah di kolom title CMS. Mulai dengan satu atau lebih paragraf <p> pembuka, lalu isi memakai <h2>, <h3>, <h4>, <ul>, <ol>, <li>, <strong>, <u>, <em>, <blockquote>, <table> bila perlu.

3. Kesimpulan: Berikan kesimpulan atau penutup yang kuat dan informatif di akhir artikel yang merangkum poin utama dan memperkuat nilai yang ditawarkan kepada pembaca terkait {{title}}.
4. FAQ: Sertakan bagian FAQ di akhir yang menjawab pertanyaan umum terkait {{title}}. Ini menambah nilai bagi pembaca dan meningkatkan SEO dengan mengakomodasi long-tail keyword queries.

Pastikan {{content}}:

- Unik: Tidak ada duplikasi {{content}} dari internet.
- Relevan: Semua informasi harus relevan dengan {{title}} dan mencerminkan User Intent.
- Mudah Dipahami: Gunakan bahasa yang sederhana dan jelas.
- Terkini: Informasi harus sesuai dengan tren dan data terkini.
-Tidak terdeteksi sebagai artikel hasil AI.

Fokuskan artikel pada memberikan nilai dan informasi yang berguna bagi pembaca yang tertarik pada {{title}}. Gunakan penelitian dan data yang relevan untuk mendukung poin dan klaim dalam artikel. Jelaskan bagaimana {{title}} relevan dengan kebutuhan atau masalah spesifik yang dihadapi oleh pembaca, dan bagaimana artikel Anda menawarkan solusi atau wawasan.

Catatan: Tolong pastikan bahwa hasilnya unik dan belum pernah diterbitkan di tempat lain di internet.
Jangan gunakan subjeck pada artikel seperti anda, kamu dan sebagainya. Ini salah satu sumbernya (masukan artikel sumber)
`

const defaultGenerateImagePrompt = `Buat gambar cover artikel berkualitas sedang dengan rasio horizontal (landscape) yang umum untuk artikel, utamakan 3:2 (setara 1024×683 px), berdasarkan judul: "{{title}}".

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

const defaultGenerateImageToContentPrompt = `Buat ilustrasi artikel berkualitas sedang untuk disisipkan di tengah isi tulisan (bukan thumbnail cover), rasio landscape atau netral (utamakan 3:2 atau 16:9), berdasarkan judul: "{{title}}".

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

func pickGeneratePromptContentTemplate(row *entity.AISetting, variant int) string {
	if variant < 1 || variant > 3 {
		variant = 1
	}
	if row == nil {
		return defaultGeneratePromptContent
	}
	switch variant {
	case 2:
		if v := strings.TrimSpace(row.GeneratePromptContent2); v != "" {
			return v
		}
	case 3:
		if v := strings.TrimSpace(row.GeneratePromptContent3); v != "" {
			return v
		}
	}
	if v := strings.TrimSpace(row.GeneratePromptContent); v != "" {
		return v
	}
	return defaultGeneratePromptContent
}

var (
	bulkPlaceholderRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_]+)\s*\}\}`)
	slugifyAlnumPost  = regexp.MustCompile(`[^a-z0-9]+`)
)

const (
	postImageMaxBytes = 5 * 1024 * 1024
	postImageDir      = "public/static/uploads/posts"
	postImageURLBase  = "/static/uploads/posts"
	postImageSlugLen  = 220
	// Manual generate-image must not use c.Request.Context(): slow providers + reverse-proxy
	// idle timeouts cancel the request context and abort the OpenAI/Gemini call with "context canceled".
	manualPostGenerateImageTimeout = 3 * time.Minute
)

var postImageSlugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// PostHandler struct
type PostHandler struct {
	svc        services.PostService
	ai         *ai.Generator
	cfg        appconfig.Config
	aiSet      repositories.AISettingRepository
	db         *gorm.DB
	autoPoster *social.AutoPoster
	bulkJobsMu sync.RWMutex
	bulkJobs   map[string]*bulkGenerateJob
	bulkQueue  chan *bulkGenerateJob
	bulkOnce   sync.Once
}

// function to create a new PostHandler
func NewPostHandler(svc services.PostService, aiGen *ai.Generator, cfg appconfig.Config, aiSet repositories.AISettingRepository, db *gorm.DB, autoPoster *social.AutoPoster) *PostHandler {
	return &PostHandler{svc: svc, ai: aiGen, cfg: cfg, aiSet: aiSet, db: db, autoPoster: autoPoster}
}

// List
// @Summary      List posts
// @Description  Get posts with pagination
// @Tags         posts
// @Produce      json
// @Param        page  query     int  false  "Page number" default(1)
// @Param        size  query     int  false  "Page size (max 500)" default(10)
// @Success      200  {object}  dto.PaginatedPostsResponse
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/posts [get]
func (h *PostHandler) List(c *gin.Context) {
	page, size := parsePagination(c)
	q := strings.TrimSpace(c.Query("q"))
	status := strings.TrimSpace(c.Query("status"))
	categoryID := parseUintQuery(c.Query("category_id"))
	authorID := parseUintQuery(c.Query("author_id"))
	requesterID, requesterRole := requesterFromContext(c)
	if requesterRole == "writer" {
		authorID = requesterID
	}

	var (
		items []entity.Post
		total int64
		err   error
	)
	if requesterRole == "writer" {
		items, total, err = h.svc.ListByAuthor(page, size, requesterID, q, status, categoryID, authorID)
	} else {
		items, total, err = h.svc.List(page, size, q, status, categoryID, authorID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"page":        page,
		"size":        size,
		"total":       total,
		"total_pages": totalPages(total, size),
	})
}

func parseUintQuery(raw string) uint {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0
	}
	return uint(n)
}

// ListPublished returns published posts only (public client API).
func (h *PostHandler) ListPublished(c *gin.Context) {
	page, size := parsePublicPostsPagination(c)
	skip := parsePublicSkip(c)
	categorySlug := strings.TrimSpace(c.Query("category_slug"))
	excludeCategorySlug := strings.TrimSpace(c.Query("exclude_category_slug"))

	items, total, err := h.svc.ListPublished(page, size, skip, categorySlug, excludeCategorySlug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":       items,
		"page":        page,
		"size":        size,
		"skip":        skip,
		"total":       total,
		"total_pages": totalPagesWithSkip(total, skip, size),
	})
}

// GetByID
// @Summary      Get post by ID
// @Description  Get a single post by ID
// @Tags         posts
// @Produce      json
// @Param        id   path      int  true  "Post ID"
// @Success      200  {object}  entity.Post
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/posts/{id} [get]
func (h *PostHandler) GetByID(c *gin.Context) {
	// get the post by ID
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.svc.Get(uint(id))
	if err != nil {
		respondNotFound(c, err)
		return
	}
	requesterID, requesterRole := requesterFromContext(c)
	if requesterRole == "writer" && item.AuthorID != requesterID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke post ini"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// GetPublishedBySlug returns a single published post by slug (public client API).
// View records: at most one per client IP per UTC day (see post_views).
func (h *PostHandler) GetPublishedBySlug(c *gin.Context) {
	slug := c.Param("slug")
	item, err := h.svc.GetPublishedBySlug(slug)
	if err != nil {
		respondNotFound(c, err)
		return
	}
	_, _ = h.svc.RecordPublicView(item.ID, c.ClientIP())
	if vc, err := h.svc.CountPostViews(item.ID); err == nil {
		item.View = vc
	}

	siteSetting := repositories.SiteSettingOrDefault(h.db)
	baseURL := publicCanonicalSiteBaseURL(c)
	displayTitle := strings.TrimSpace(item.Title)
	item.Content = posthtml.BuildPublicPostBodyHTML(baseURL, item.Content, displayTitle, siteSetting)
	if siteSetting.AutoHrefEnabled {
		item.Content = autohref.Apply(h.db, siteSetting, item.ID, item.Content)
	}

	c.JSON(http.StatusOK, item)
}

func publicCanonicalSiteBaseURL(c *gin.Context) string {
	proto := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		proto = "https"
	}
	return proto + "://" + c.Request.Host
}

// ListPopular returns popular published posts by unique views in the last N hours (default 24).
func (h *PostHandler) ListPopular(c *gin.Context) {
	hours := 24
	if v := strings.TrimSpace(c.Query("hours")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 24*30 {
			hours = n
		}
	}
	limit := 5
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	items, err := h.svc.PopularPublishedSince(cutoff, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GenerateDraft uses the configured AI provider to produce HTML for a new post body.
// @Summary      Generate post draft with AI
// @Description  Returns HTML body fragment (no <h1>; title is a separate field) from OpenAI, Groq, or Gemini based on prompt and provider.
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        body  body      dto.GeneratePostDraftRequest  true  "Prompt and optional provider"
// @Success      200   {object}  map[string]interface{}  "content (HTML), meta_description, meta_keywords, ai_requests (provider calls), word_count (plain text)"
// @Failure      400   {object}  map[string]string
// @Failure      429   {object}  map[string]string
// @Failure      502   {object}  map[string]string
// @Failure      503   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/posts/generate-draft [post]
func (h *PostHandler) GenerateDraft(c *gin.Context) {
	if h.ai == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generator AI belum dikonfigurasi"})
		return
	}

	var req dto.GeneratePostDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	userPrompt := strings.TrimSpace(req.Prompt)
	if userPrompt == "" {
		title := strings.TrimSpace(req.Title)
		if utf8.RuneCountInString(title) < 3 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Judul wajib diisi jika prompt kosong"})
			return
		}
		tpl := defaultGeneratePromptContent
		if h.aiSet != nil {
			if row, err := h.aiSet.GetOrCreate(); err == nil && row != nil {
				tpl = pickGeneratePromptContentTemplate(row, req.PromptContentVariant)
			}
		}
		userPrompt = replaceBulkPostPlaceholders(tpl, bulkPlaceholderValues(
			title,
			title,
			"Kembangkan isi yang informatif dan SEO-friendly berdasarkan judul dan kata kunci utama.",
			"Ringkas meta description (sekitar 150–160 karakter) yang relevan dengan judul.",
		))
	}

	p := strings.ToLower(strings.TrimSpace(req.Provider))
	if p == "" {
		p = "gemini"
	}

	fullPrompt := blogDraftSystemPrompt + userPrompt

	ctx := c.Request.Context()
	text, usedProv, err := h.generateAIText(ctx, p, fullPrompt)
	if err != nil {
		h.persistManualPostAILog(c, "error", "generate_draft_err", err.Error(), nil, req.Title, p, usedProv)
		h.respondAIError(c, err, p)
		return
	}
	text = strings.TrimSpace(text)
	text = posthtml.NormalizeParagraphRows(text)
	aiCalls := 1
	plainBody := postmeta.PlainExcerpt(text, 0)
	wordCount := len(strings.Fields(plainBody))

	metaProv := strings.TrimSpace(usedProv)
	if metaProv == "" {
		metaProv = p
	}

	titleLine := strings.TrimSpace(req.Title)
	tagNames := h.draftOrderedTagNames(ctx, req.Tags)
	tagKWCSV := strings.Join(tagNames, ", ")
	excerpt := postmeta.PlainExcerpt(text, 4000)
	if titleLine == "" {
		titleLine = strings.TrimSpace(postmeta.PlainExcerpt(text, 120))
		if titleLine == "" {
			titleLine = "Blog article"
		}
	}

	metaDescTpl := defaultGeneratePromptMetaDescription
	metaKwTpl := defaultGeneratePromptMetaKeywords
	if h.aiSet != nil {
		if row, err2 := h.aiSet.GetOrCreate(); err2 == nil && row != nil {
			if v := strings.TrimSpace(row.GeneratePromptMetaDescription); v != "" {
				metaDescTpl = v
			}
			if v := strings.TrimSpace(row.GeneratePromptMetaKeywords); v != "" {
				metaKwTpl = v
			}
		}
	}
	vals := bulkPlaceholderValues(titleLine, tagKWCSV, excerpt, "")
	descUser := replaceBulkPostPlaceholders(metaDescTpl, vals)
	kwUser := replaceBulkPostPlaceholders(metaKwTpl, vals)

	metaDesc := ""
	metaKW := ""
	if md, _, err2 := h.generateAIText(ctx, metaProv, blogDraftMetaDescriptionSystemPrompt+descUser); err2 == nil {
		metaDesc = sanitizeDraftMetaDescription(md)
		aiCalls++
	}
	if mk, _, err2 := h.generateAIText(ctx, metaProv, blogDraftMetaKeywordsSystemPrompt+kwUser); err2 == nil {
		metaKW = sanitizeDraftMetaKeywordsCSV(mk)
		aiCalls++
	}

	tmp := &entity.Post{Title: titleLine, Content: text}
	postmeta.Apply(tmp, tagNames, "")
	if metaDesc == "" && tmp.MetaDesc != nil {
		metaDesc = strings.TrimSpace(*tmp.MetaDesc)
	}
	if metaKW == "" && tmp.MetaKeywords != nil {
		metaKW = strings.TrimSpace(*tmp.MetaKeywords)
	}
	h.persistManualPostAILog(
		c,
		"info",
		"generate_draft_ok",
		fmt.Sprintf("permintaan_ai=%d jumlah_kata=%d", aiCalls, wordCount),
		nil,
		req.Title,
		p,
		usedProv,
	)

	c.JSON(http.StatusOK, gin.H{
		"content":          text,
		"meta_description": metaDesc,
		"meta_keywords":    metaKW,
		"ai_requests":      aiCalls,
		"word_count":       wordCount,
	})
}

// GenerateImage generates or searches a post image and stores it in upload storage.
func (h *PostHandler) GenerateImage(c *gin.Context) {
	var req dto.GeneratePostImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "" {
		source = "ai"
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "auto"
	}
	slug, err := h.ensureUniquePostSlug(req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan slug post"})
		return
	}
	imgCtx, imgCancel := context.WithTimeout(context.Background(), manualPostGenerateImageTimeout)
	defer imgCancel()
	forContent := strings.EqualFold(strings.TrimSpace(req.Purpose), "content")
	purpose := imageAIPurposeLabel(forContent)
	if source == "web_search" {
		if forContent {
			purpose = "content"
		}
		store := storage.LoadR2ObjectStore(imgCtx, h.db, h.cfg)
		attempt := req.SearchAttempt
		if attempt < 0 {
			attempt = 0
		}
		searchSlug := slug
		if attempt > 0 {
			searchSlug = fmt.Sprintf("%s-web-%d", slug, attempt+1)
		}
		webSource := strings.ToLower(strings.TrimSpace(req.WebSearchSource))
		if webSource == "" {
			webSource = "auto"
		}
		if imageRes := scraper.BulkCoverImageResultFromWebSourcesAttemptSource(imgCtx, h.db, store, req.Title, searchSlug, attempt, webSource); imageRes != nil && strings.TrimSpace(imageRes.StoredURL) != "" {
			imageURL := strings.TrimSpace(imageRes.StoredURL)
			sourceURL := strings.TrimSpace(imageRes.SourceURL)
			prevImage := strings.TrimSpace(req.PreviousImage)
			if prevImage != "" && prevImage != imageURL {
				deleteStore := storage.PostAssetStore(imgCtx, h.db, h.cfg)
				_ = deleteStore.DeleteURL(imgCtx, prevImage)
			}
			h.persistManualPostAILog(c, "info", "generate_image_ok", imageAILogMessage(purpose, "web_search", imageURL), nil, req.Title, "web_search", "web_search")
			detail := strings.TrimSpace(imageRes.Provider)
			if detail == "" && webSource != "" && webSource != "auto" {
				detail = webSource
			}
			c.JSON(http.StatusOK, gin.H{
				"image":               imageURL,
				"provider_used":       "web_search",
				"search_attempt":      attempt,
				"web_search_source":   webSource,
				"image_source_url":    sourceURL,
				"image_origin":        entity.ImageOriginWebSearch,
				"image_origin_detail": detail,
			})
			return
		}
		err := errors.New("Gambar web tidak ditemukan")
		h.persistManualPostAILog(c, "error", "generate_image_err", imageAILogMessage(purpose, "web_search", "")+" err="+err.Error(), nil, req.Title, "web_search", "web_search")
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if h.ai == nil || h.aiSet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generator AI belum dikonfigurasi"})
		return
	}
	imageURL, used, err := h.generateAICoverImage(imgCtx, req.Title, req.Prompt, provider, slug, forContent)
	if err != nil {
		msg := imageAILogMessage(purpose, imageAIProviderForLog(used, provider), "") + " err=" + err.Error()
		h.persistManualPostAILog(c, "error", "generate_image_err", msg, nil, req.Title, provider, used)
		h.respondAIError(c, err, provider)
		return
	}
	h.persistManualPostAILog(c, "info", "generate_image_ok", imageAILogMessage(purpose, used, imageURL), nil, req.Title, provider, used)
	c.JSON(http.StatusOK, gin.H{
		"image":         imageURL,
		"provider_used": used,
		"image_origin":  entity.ImageOriginAI,
	})
}

// BulkSuggestTitles returns AI-generated title lines from a keyword (one title per line).
func (h *PostHandler) BulkSuggestTitles(c *gin.Context) {
	if h.ai == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generator AI belum dikonfigurasi"})
		return
	}
	var req dto.BulkSuggestTitlesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	p := strings.ToLower(strings.TrimSpace(req.Provider))
	if p == "" {
		p = "auto"
	}
	titleTpl := bulkTitlesPromptPrefix
	if h.aiSet != nil {
		if row, err := h.aiSet.GetOrCreate(); err == nil && row != nil {
			if t := strings.TrimSpace(row.GeneratePromptTitle); t != "" {
				titleTpl = t
				if !strings.HasSuffix(titleTpl, "\n") {
					titleTpl += "\n"
				}
			}
		}
	}
	prompt := titleTpl + fmt.Sprintf("\nKeyword/theme: %q\nReturn exactly %d different article titles.", strings.TrimSpace(req.Keyword), req.Count)
	uid, _ := requesterFromContext(c)
	t0 := time.Now()
	log.Printf("[bulk-post] AI API suggest_titles start user_id=%d keyword=%q count=%d", uid, strings.TrimSpace(req.Keyword), req.Count)
	ctx := c.Request.Context()
	text, effective, err := h.generateAIText(ctx, p, prompt)
	if err != nil {
		log.Printf("[bulk-post] AI API suggest_titles failed user_id=%d provider_used=%s duration_ms=%d err=%v", uid, effective, time.Since(t0).Milliseconds(), err)
		h.persistBulkPostLog(c, "error", "suggest_titles_err", err.Error(), nil, nil, "", effective)
		h.respondAIError(c, err, p)
		return
	}
	trimmed := strings.TrimSpace(text)
	lines := 0
	for _, line := range strings.Split(trimmed, "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	log.Printf("[bulk-post] AI API suggest_titles ok user_id=%d provider_used=%s duration_ms=%d title_lines=%d", uid, effective, time.Since(t0).Milliseconds(), lines)
	h.persistBulkPostLog(c, "info", "suggest_titles_ok", fmt.Sprintf("kata_kunci=%q baris_judul=%d", strings.TrimSpace(req.Keyword), lines), nil, nil, "", effective)
	c.JSON(http.StatusOK, gin.H{"titles_text": trimmed})
}

func splitBulkTitleLinesRaw(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

func normalizeBulkRegeneratedTitleLines(ai string, want int) []string {
	s := strings.ReplaceAll(strings.TrimSpace(ai), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	parts := strings.Split(s, "\n")
	out := make([]string, want)
	for i := 0; i < want; i++ {
		ln := ""
		if i < len(parts) {
			ln = strings.TrimSpace(parts[i])
		}
		if utf8.RuneCountInString(ln) > 220 {
			ln = string([]rune(ln)[:220])
		}
		out[i] = ln
	}
	return out
}

func buildBulkRegenerateTitlesPrompt(regenPrefix, hint string, chunkLines []string) string {
	var numbered strings.Builder
	for i, ln := range chunkLines {
		fmt.Fprintf(&numbered, "%d. %s\n", i+1, ln)
	}
	k := len(chunkLines)
	return regenPrefix + hint + fmt.Sprintf("There are exactly %d input lines below. Return exactly %d output lines, same order.\n\nINPUT:\n", k, k) + numbered.String()
}

// BulkRegenerateTitles rewrites each line in titles_text (same line count) via AI.
func (h *PostHandler) BulkRegenerateTitles(c *gin.Context) {
	if h.ai == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Generator AI belum dikonfigurasi"})
		return
	}
	var req dto.BulkRegenerateTitlesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	inLines := splitBulkTitleLinesRaw(req.TitlesText)
	if len(inLines) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Judul artikel kosong"})
		return
	}
	if len(inLines) > bulkMaxArticleTitleLines {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Maksimal %d baris", bulkMaxArticleTitleLines)})
		return
	}
	nonEmpty := 0
	for _, ln := range inLines {
		if strings.TrimSpace(ln) != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Masukkan minimal satu baris judul"})
		return
	}
	K := len(inLines)
	p := strings.ToLower(strings.TrimSpace(req.Provider))
	if p == "" {
		p = "auto"
	}
	hint := ""
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		hint = fmt.Sprintf("Optional theme/hint for the whole batch: %q\n\n", kw)
	}
	regenPrefix := bulkRegenerateTitlesPromptPrefix
	if h.aiSet != nil {
		if row, err := h.aiSet.GetOrCreate(); err == nil && row != nil {
			if t := strings.TrimSpace(row.GeneratePromptTitle); t != "" {
				regenPrefix = t
				if !strings.HasSuffix(regenPrefix, "\n") {
					regenPrefix += "\n"
				}
				regenPrefix += "\n" + bulkRegenerateTitlesPromptPrefix
			}
		}
	}
	uid, _ := requesterFromContext(c)
	t0 := time.Now()
	nChunks := (K + bulkRegenerateLinesPerAPICall - 1) / bulkRegenerateLinesPerAPICall
	log.Printf("[bulk-post] AI API regenerate_titles start user_id=%d lines=%d chunks=%d", uid, K, nChunks)
	ctx := c.Request.Context()
	allOut := make([]string, 0, K)
	effective := ""
	lockedProv := ""
	for start := 0; start < K; start += bulkRegenerateLinesPerAPICall {
		end := start + bulkRegenerateLinesPerAPICall
		if end > K {
			end = K
		}
		chunk := inLines[start:end]
		ck := len(chunk)
		prompt := buildBulkRegenerateTitlesPrompt(regenPrefix, hint, chunk)
		callProv := p
		if p == "auto" && lockedProv != "" {
			callProv = lockedProv
		}
		text, used, err := h.generateAIText(ctx, callProv, prompt)
		if err != nil {
			usedStr := strings.ToLower(strings.TrimSpace(used))
			if usedStr == "" {
				usedStr = callProv
			}
			log.Printf("[bulk-post] AI API regenerate_titles failed user_id=%d chunk=%d-%d/%d provider_used=%s duration_ms=%d err=%v", uid, start+1, end, K, usedStr, time.Since(t0).Milliseconds(), err)
			h.persistBulkPostLog(c, "error", "regenerate_titles_err", err.Error(), nil, nil, "", usedStr)
			h.respondAIError(c, err, p)
			return
		}
		if p == "auto" && lockedProv == "" {
			lockedProv = strings.ToLower(strings.TrimSpace(used))
		}
		effective = strings.ToLower(strings.TrimSpace(used))
		outChunk := normalizeBulkRegeneratedTitleLines(text, ck)
		allOut = append(allOut, outChunk...)
	}
	joined := strings.Join(allOut, "\n")
	log.Printf("[bulk-post] AI API regenerate_titles ok user_id=%d provider_used=%s duration_ms=%d lines=%d", uid, effective, time.Since(t0).Milliseconds(), K)
	h.persistBulkPostLog(c, "info", "regenerate_titles_ok", fmt.Sprintf("baris=%d bagian=%d", K, nChunks), nil, nil, "", effective)
	c.JSON(http.StatusOK, gin.H{"titles_text": joined, "count": K})
}

// BulkScrapeXMLTitles fills the same title list shape as AI suggest, from RSS/Atom/WXR XML (paste or fetch by URL).
func (h *PostHandler) BulkScrapeXMLTitles(c *gin.Context) {
	var req dto.BulkScrapeXMLTitlesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	u := strings.TrimSpace(req.URL)
	x := strings.TrimSpace(req.XML)
	if u == "" && x == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Isi URL feed atau tempel XML."})
		return
	}
	uid, _ := requesterFromContext(c)
	t0 := time.Now()
	var titles []string
	var err error
	if u != "" {
		log.Printf("[bulk-post] scrape_xml_titles start user_id=%d via=url", uid)
		titles, err = feedtitles.FetchAndExtract(nil, u)
	} else {
		log.Printf("[bulk-post] scrape_xml_titles start user_id=%d via=body len=%d", uid, len(x))
		titles, err = feedtitles.ExtractFromBytes([]byte(x))
	}
	if err != nil {
		log.Printf("[bulk-post] scrape_xml_titles failed user_id=%d duration_ms=%d err=%v", uid, time.Since(t0).Milliseconds(), err)
		h.persistBulkPostLog(c, "warn", "scrape_xml_titles_err", err.Error(), nil, nil, "", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data post tidak valid"})
		return
	}
	joined := strings.TrimSpace(strings.Join(titles, "\n"))
	log.Printf("[bulk-post] scrape_xml_titles ok user_id=%d duration_ms=%d titles=%d", uid, time.Since(t0).Milliseconds(), len(titles))
	h.persistBulkPostLog(c, "info", "scrape_xml_titles_ok", fmt.Sprintf("judul=%d", len(titles)), nil, nil, "", "")
	c.JSON(http.StatusOK, gin.H{"titles_text": joined, "count": len(titles)})
}

// BulkGeneratePosts creates one post per title using the bulk prompt template from AI settings.
func (h *PostHandler) BulkGeneratePosts(c *gin.Context) {
	if h.ai == nil || h.aiSet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI belum dikonfigurasi"})
		return
	}
	var req dto.BulkGeneratePostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	req.Titles = trimTrailingBlankBulkStrings(req.Titles)
	req.Slugs = trimTrailingBlankBulkStrings(req.Slugs)
	req.CategoryRefs = trimTrailingBlankBulkStrings(req.CategoryRefs)
	if len(req.Slugs) > 0 && len(req.Slugs) != len(req.Titles) {
		h.persistBulkPostLog(c, "warn", "generate_abort", "Jumlah slug tidak sama", nil, nil, "", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jumlah slug harus sama dengan jumlah judul (satu per baris)"})
		return
	}
	if len(req.CategoryIDs) > 0 && len(req.CategoryIDs) != len(req.Titles) {
		h.persistBulkPostLog(c, "warn", "generate_abort", "Jumlah ID kategori tidak sama", nil, nil, "", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jumlah category_ids harus sama dengan jumlah judul (satu per baris)"})
		return
	}
	if len(req.CategoryRefs) > 0 && len(req.CategoryRefs) != len(req.Titles) {
		h.persistBulkPostLog(c, "warn", "generate_abort", "Jumlah referensi kategori tidak sama", nil, nil, "", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Jumlah category_refs harus sama dengan jumlah judul (satu per baris)"})
		return
	}
	hadSlugs := len(req.Slugs) > 0
	hadCategoryIDs := len(req.CategoryIDs) > 0 || len(req.CategoryRefs) > 0
	categoryIDsFromRefs := req.CategoryIDs
	if len(req.CategoryRefs) > 0 {
		resolved, err := h.resolveBulkCategoryRefs(req.CategoryRefs)
		if err != nil {
			h.persistBulkPostLog(c, "warn", "generate_abort", err.Error(), nil, nil, "", "")
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		categoryIDsFromRefs = resolved
	}

	cleanTitles := make([]string, 0, len(req.Titles))
	cleanSlugs := make([]string, 0, len(req.Titles))
	cleanCategoryIDs := make([]uint, 0, len(req.Titles))
	seen := make(map[string]struct{}, len(req.Titles))
	for i, t := range req.Titles {
		t = strings.TrimSpace(t)
		if t == "" || len(t) > 220 {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleanTitles = append(cleanTitles, t)
		if hadSlugs {
			s := strings.TrimSpace(req.Slugs[i])
			if s != "" {
				sNorm := slugifyPostTitleForBulk(s)
				if sNorm == "" || utf8.RuneCountInString(sNorm) < 3 || utf8.RuneCountInString(sNorm) > 255 || !postImageSlugRe.MatchString(sNorm) {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Slug tidak valid untuk judul %q (baris %d): gunakan 3-255 karakter, huruf kecil, angka, dan tanda hubung saja", t, i+1)})
					return
				}
				cleanSlugs = append(cleanSlugs, sNorm)
			} else {
				cleanSlugs = append(cleanSlugs, "")
			}
		}
		if hadCategoryIDs {
			cleanCategoryIDs = append(cleanCategoryIDs, categoryIDsFromRefs[i])
		}
	}
	if len(cleanTitles) == 0 {
		h.persistBulkPostLog(c, "warn", "generate_abort", "Tidak ada judul yang valid", nil, nil, "", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada judul yang valid"})
		return
	}
	if len(cleanTitles) > bulkMaxArticleTitleLines {
		h.persistBulkPostLog(c, "warn", "generate_abort", fmt.Sprintf("Maksimal %d judul", bulkMaxArticleTitleLines), nil, nil, "", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Maksimal %d judul", bulkMaxArticleTitleLines)})
		return
	}
	if !hadCategoryIDs && req.CategoryID == 0 {
		h.persistBulkPostLog(c, "warn", "generate_abort", "Kategori belum dipilih", nil, nil, "", "")
		c.JSON(http.StatusBadRequest, gin.H{"error": "category_id wajib diisi kecuali category_ids tersedia"})
		return
	}

	requesterID, requesterRole := requesterFromContext(c)
	authorID := req.AuthorID
	if requesterRole == "writer" {
		authorID = requesterID
	}

	reqStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if reqStatus == "" {
		reqStatus = "draft"
	}
	if reqStatus != "draft" && reqStatus != "published" && reqStatus != "scheduled" {
		reqStatus = "draft"
	}

	req.AuthorID = authorID
	req.Status = reqStatus
	req.Titles = cleanTitles
	if hadSlugs {
		req.Slugs = cleanSlugs
	} else {
		req.Slugs = nil
	}
	if hadCategoryIDs {
		req.CategoryIDs = cleanCategoryIDs
		req.CategoryID = 0
	} else {
		req.CategoryIDs = nil
	}

	job, err := h.enqueueBulkGenerateJob(requesterID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat antrean bulk post"})
		return
	}
	log.Printf("[bulk-post] AI bulk generate job queued job_id=%s user_id=%d titles=%d category_id=%d per_category=%t author_id=%d status=%s", job.ID, requesterID, len(cleanTitles), req.CategoryID, len(req.CategoryIDs) > 0, authorID, reqStatus)
	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  job.ID,
		"status":  job.Status,
		"total":   job.Total,
		"queued":  true,
		"message": "bulk generate job queued",
	})
}

func (h *PostHandler) resolveBulkCategoryRefs(refs []string) ([]uint, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	if h.db == nil {
		return nil, errors.New("Database belum dikonfigurasi")
	}
	var cats []entity.Category
	if err := h.db.Select("id", "name", "slug").Find(&cats).Error; err != nil {
		return nil, err
	}
	byRef := make(map[string]uint, len(cats)*3)
	for _, cat := range cats {
		if cat.ID == 0 {
			continue
		}
		byRef[strconv.FormatUint(uint64(cat.ID), 10)] = cat.ID
		if name := normalizeBulkCategoryRef(cat.Name); name != "" {
			byRef[name] = cat.ID
		}
		if slug := normalizeBulkCategoryRef(cat.Slug); slug != "" {
			byRef[slug] = cat.ID
		}
	}
	out := make([]uint, 0, len(refs))
	for i, raw := range refs {
		key := normalizeBulkCategoryRef(raw)
		if key == "" {
			return nil, fmt.Errorf("baris %d: kategori wajib diisi", i+1)
		}
		id, ok := byRef[key]
		if !ok {
			return nil, fmt.Errorf("baris %d: kategori %q tidak ditemukan", i+1, strings.TrimSpace(raw))
		}
		out = append(out, id)
	}
	return out, nil
}

func normalizeBulkCategoryRef(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func trimTrailingBlankBulkStrings(items []string) []string {
	for len(items) > 0 && strings.TrimSpace(items[len(items)-1]) == "" {
		items = items[:len(items)-1]
	}
	return items
}

// BulkGenerateJobStatus returns queue worker progress/result for one bulk generate job.
func (h *PostHandler) BulkGenerateJobStatus(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID job tidak ditemukan"})
		return
	}
	requesterID, requesterRole := requesterFromContext(c)
	job, ok := h.getBulkGenerateJob(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job tidak ditemukan"})
		return
	}
	if requesterRole == "writer" && job.UserID != requesterID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke job ini"})
		return
	}
	c.JSON(http.StatusOK, job.toResponse())
}

// ActiveBulkGenerateJob returns the newest queued/running bulk generation job visible to the requester.
func (h *PostHandler) ActiveBulkGenerateJob(c *gin.Context) {
	requesterID, requesterRole := requesterFromContext(c)
	job, ok := h.getActiveBulkGenerateJob(requesterID, requesterRole)
	if !ok || job == nil {
		c.JSON(http.StatusOK, gin.H{"active": false})
		return
	}
	res := job.toResponse()
	res["active"] = true
	c.JSON(http.StatusOK, res)
}

// CancelBulkGenerateJob force-stops a queued/running bulk post generation job.
func (h *PostHandler) CancelBulkGenerateJob(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("id"))
	requesterID, requesterRole := requesterFromContext(c)
	job, ok, err := h.cancelBulkGenerateJob(jobID, requesterID, requesterRole)
	if err != nil {
		if strings.EqualFold(err.Error(), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke job ini"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !ok || job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job tidak ditemukan"})
		return
	}
	h.persistBulkPostLogByUser(job.UserID, "warn", "generate_cancel_requested", "Permintaan pembatalan generate bulk diterima", nil, nil, "", "")
	c.JSON(http.StatusOK, job.toResponse())
}

type bulkGenerateAIResult struct {
	Content         string `json:"content"`
	MetaDescription string `json:"meta_description"`
	MetaKeywords    string `json:"meta_keywords"`
}

func parseBulkGenerateJSON(raw string) (*bulkGenerateAIResult, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, errors.New("Respons AI kosong")
	}
	candidates := make([]string, 0, 4)
	candidates = append(candidates, s)
	if unwrapped := unwrapFencedJSON(s); unwrapped != "" && unwrapped != s {
		candidates = append(candidates, unwrapped)
	}
	if obj := firstJSONObject(s); obj != "" {
		candidates = append(candidates, obj)
	}
	if unwrapped := unwrapFencedJSON(s); unwrapped != "" {
		if obj := firstJSONObject(unwrapped); obj != "" {
			candidates = append(candidates, obj)
		}
	}

	var out bulkGenerateAIResult
	for _, cand := range candidates {
		if err := json.Unmarshal([]byte(cand), &out); err == nil {
			return &out, nil
		}
	}
	return nil, errors.New("Respons JSON AI tidak valid")
}

func unwrapFencedJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	start := 0
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		start = 1
	}
	end := len(lines)
	for end > start {
		if strings.HasPrefix(strings.TrimSpace(lines[end-1]), "```") {
			end--
			break
		}
		end--
	}
	if end <= start {
		return s
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func firstJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	inString := false
	escaped := false
	depth := 0
	start := -1
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
			continue
		}
		if ch == '}' {
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					return strings.TrimSpace(s[start : i+1])
				}
			}
		}
	}
	return ""
}

// bulkTagsAppendKeywordsCSV appends tags parsed from comma-separated keyword text.
func (h *PostHandler) bulkTagsAppendKeywordsCSV(keywordCSV string, base []entity.Tag) ([]entity.Tag, error) {
	if h.db == nil {
		return base, nil
	}
	seenNames := make(map[string]struct{}, 16)
	keywords := make([]string, 0, 8)
	for _, raw := range strings.Split(keywordCSV, ",") {
		kw := firstTwoWordsTagLabel(raw)
		if kw == "" {
			continue
		}
		key := strings.ToLower(kw)
		if _, ok := seenNames[key]; ok {
			continue
		}
		seenNames[key] = struct{}{}
		keywords = append(keywords, kw)
	}
	if len(keywords) == 0 {
		return base, nil
	}
	seenTagIDs := make(map[uint]struct{}, len(base))
	for _, t := range base {
		if t.ID != 0 {
			seenTagIDs[t.ID] = struct{}{}
		}
	}
	out := append([]entity.Tag(nil), base...)
	for _, kw := range keywords {
		tagID, err := h.createOrFindTagIDByKeyword(kw)
		if err != nil {
			return nil, err
		}
		if _, ok := seenTagIDs[tagID]; ok {
			continue
		}
		seenTagIDs[tagID] = struct{}{}
		out = append(out, entity.Tag{ID: tagID})
	}
	return out, nil
}

func (h *PostHandler) createOrFindTagIDByKeyword(kw string) (uint, error) {
	kw = firstTwoWordsTagLabel(kw)
	if kw == "" {
		return 0, errors.New("Kata kunci tag kosong")
	}
	var existing entity.Tag
	if err := h.db.Where("LOWER(TRIM(name)) = ?", strings.ToLower(kw)).First(&existing).Error; err == nil {
		return existing.ID, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	slugBase := slugifyPostTitleForBulk(kw)
	if slugBase == "" {
		slugBase = "tag"
	}
	var tagID uint
	for i := 0; i < 500; i++ {
		candidate := slugBase
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", slugBase, i+1)
		}
		newT := entity.Tag{Name: kw, Slug: candidate}
		if err := h.db.Create(&newT).Error; err == nil {
			tagID = newT.ID
			break
		}
	}
	if tagID == 0 {
		if err := h.db.Where("LOWER(TRIM(name)) = ?", strings.ToLower(kw)).First(&existing).Error; err != nil {
			return 0, err
		}
		tagID = existing.ID
	}
	return tagID, nil
}

func firstTwoWordsTagLabel(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	if len(parts) <= 2 {
		return s
	}
	return parts[0] + " " + parts[1]
}

// bulkPlaceholderValues maps English placeholders used in templates; legacy Indonesian names are included for older saved templates.
func bulkPlaceholderValues(titleLine, keywords, contentHint, metaDescHint string) map[string]string {
	return map[string]string{
		"title":            titleLine,
		"keywords":         keywords,
		"content":          contentHint,
		"meta_description": metaDescHint,
		"judul":            titleLine,
		"kata_kunci":       keywords,
		"isi":              contentHint,
		"deskripsi":        metaDescHint,
	}
}

func replaceBulkPostPlaceholders(tpl string, vals map[string]string) string {
	return bulkPlaceholderRe.ReplaceAllStringFunc(tpl, func(orig string) string {
		sub := bulkPlaceholderRe.FindStringSubmatch(orig)
		if len(sub) < 2 {
			return orig
		}
		key := strings.ToLower(strings.TrimSpace(sub[1]))
		if v, ok := vals[key]; ok {
			return v
		}
		return orig
	})
}

func (h *PostHandler) draftOrderedTagNames(ctx context.Context, tagIDs []uint) []string {
	if h.db == nil || len(tagIDs) == 0 {
		return nil
	}
	var tags []entity.Tag
	if err := h.db.WithContext(ctx).Where("id IN ?", tagIDs).Find(&tags).Error; err != nil {
		return nil
	}
	byID := make(map[uint]string, len(tags))
	for _, t := range tags {
		if n := strings.TrimSpace(t.Name); n != "" {
			byID[t.ID] = n
		}
	}
	out := make([]string, 0, len(tagIDs))
	for _, id := range tagIDs {
		if n, ok := byID[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

func sanitizeDraftMetaDescription(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.Join(strings.Fields(s), " ")
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "meta description:") {
		s = strings.TrimSpace(s[len("meta description:"):])
	} else if strings.HasPrefix(low, "description:") {
		s = strings.TrimSpace(s[len("description:"):])
	}
	if utf8.RuneCountInString(s) > 500 {
		s = string([]rune(s)[:500])
	}
	return strings.TrimSpace(s)
}

func sanitizeDraftMetaKeywordsCSV(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	parts := strings.Split(s, ",")
	var out []string
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	res := strings.Join(out, ", ")
	if utf8.RuneCountInString(res) > 500 {
		res = string([]rune(res)[:500])
	}
	return strings.TrimSpace(res)
}

func slugifyPostTitleForBulk(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, `"`, "")
	s = slugifyAlnumPost.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func (h *PostHandler) ensureUniquePostSlug(base string) (string, error) {
	s := slugifyPostTitleForBulk(base)
	if s == "" {
		s = "post"
	}
	for i := 0; i < 500; i++ {
		candidate := s
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", s, i+1)
		}
		taken, err := h.svc.SlugExists(candidate, 0)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", errors.New("Gagal membuat slug unik")
}

var minWordsFromPromptRe = regexp.MustCompile(`(?i)(?:minimal|min\.?)\s*(\d{3,5})\s*(?:kata|words)\b`)

var aiRefusalSnippetRe = regexp.MustCompile(`(?i)\b(i'?m sorry|i am sorry|i can'?t assist|i cannot assist|can'?t help with that|cannot help with that|i can'?t comply|i cannot comply|i'?m unable to assist|i am unable to assist)\b`)

func desiredMinWordsFromPrompt(prompt string) int {
	// Heuristic: honor explicit "minimal N kata/words" constraints inside the user prompt template.
	// Used to auto-continue OpenAI when it stops too early.
	matches := minWordsFromPromptRe.FindAllStringSubmatch(prompt, -1)
	max := 0
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	// sanity bounds
	if max < 0 {
		return 0
	}
	if max > 8000 {
		max = 8000
	}
	return max
}

func htmlWordCount(html string) int {
	plain := postmeta.PlainExcerpt(html, 0)
	return len(strings.Fields(plain))
}

func looksLikeAIRefusal(text string) bool {
	plain := strings.TrimSpace(postmeta.PlainExcerpt(text, 0))
	if plain == "" {
		return false
	}
	if utf8.RuneCountInString(plain) > 500 {
		return false
	}
	return aiRefusalSnippetRe.MatchString(plain)
}

// generateAIText returns assistant text, the effective backend provider (e.g. gemini when setting is auto), and an error.
func (h *PostHandler) generateAIText(ctx context.Context, provider, prompt string) (string, string, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch p {
	case "auto":
		text, used, err := h.ai.GenerateBlogContentWithProvider(ctx, prompt)
		return text, string(used), err
	case "openai":
		text, err := h.ai.GenerateWith(ctx, ai.ProviderOpenAI, prompt)
		if err != nil {
			return text, "openai", err
		}
		// Force OpenAI to match long-form expectations when the prompt explicitly demands a minimum word count.
		// We only do this for HTML drafts (not meta description/keywords/title suggestions).
		if strings.HasPrefix(prompt, blogDraftSystemPrompt) &&
			!strings.HasPrefix(prompt, blogDraftMetaDescriptionSystemPrompt) &&
			!strings.HasPrefix(prompt, blogDraftMetaKeywordsSystemPrompt) {
			if looksLikeAIRefusal(text) {
				retryPrompt := blogDraftSystemPrompt +
					"The previous response was a refusal. For this CMS draft, write a safe, neutral, informational article body in HTML. " +
					"Stay within allowed editorial content, avoid harmful instructions, and do not mention policies or refusal.\n\n" +
					strings.TrimPrefix(prompt, blogDraftSystemPrompt)
				if retryText, retryErr := h.ai.GenerateWith(ctx, ai.ProviderOpenAI, retryPrompt); retryErr == nil {
					text = retryText
				}
			}
			if looksLikeAIRefusal(text) {
				return text, "openai", errors.New("OpenAI menolak membuat draf ini; coba prompt yang lebih netral atau gunakan Gemini")
			}
			minW := desiredMinWordsFromPrompt(prompt)
			if minW > 0 {
				cur := strings.TrimSpace(text)
				curW := htmlWordCount(cur)
				// Up to 2 continuation calls if we are still far below the requested minimum.
				for tries := 0; tries < 2 && curW < minW; tries++ {
					contUser := "Lanjutkan artikel berikut (HTML fragment). Jangan mengulang bagian pembuka atau bagian yang sudah ada. " +
						"Targetkan total minimal " + strconv.Itoa(minW) + " kata.\n\n=== DRAFT SAAT INI ===\n" + cur
					more, err2 := h.ai.GenerateWith(ctx, ai.ProviderOpenAI, blogDraftContinuationSystemPrompt+contUser)
					if err2 != nil {
						break
					}
					more = strings.TrimSpace(more)
					if looksLikeAIRefusal(more) {
						break
					}
					if more == "" {
						break
					}
					// Avoid infinite loops if model repeats content.
					if strings.Contains(cur, more) {
						break
					}
					cur = strings.TrimSpace(cur + "\n" + more)
					nextW := htmlWordCount(cur)
					if nextW <= curW {
						break
					}
					curW = nextW
				}
				text = cur
			}
		}
		return text, "openai", nil
	case "groq":
		text, err := h.ai.GenerateWith(ctx, ai.ProviderGroq, prompt)
		return text, "groq", err
	case "gemini":
		text, err := h.ai.GenerateWith(ctx, ai.ProviderGemini, prompt)
		return text, "gemini", err
	case "claude":
		text, err := h.ai.GenerateWith(ctx, ai.ProviderClaude, prompt)
		return text, "claude", err
	default:
		return "", "", ai.ErrUnsupportedProvider
	}
}

// aiQuotaHTTPMeta maps provider quota/rate-limit errors to stable API codes for the admin UI.
func aiQuotaHTTPMeta(err error) (code, provider string, ok bool) {
	switch {
	case errors.Is(err, aiopenai.ErrQuotaOrRateLimit):
		return "openai_quota", "openai", true
	case errors.Is(err, aigroq.ErrQuotaOrRateLimit):
		return "groq_quota", "groq", true
	case errors.Is(err, aigemini.ErrQuotaOrRateLimit):
		return "gemini_quota", "gemini", true
	case errors.Is(err, aiclaud.ErrQuotaOrRateLimit):
		return "claude_quota", "claude", true
	default:
		return "", "", false
	}
}

func (h *PostHandler) respondAIError(c *gin.Context, err error, _ string) {
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
			"error": `Penyedia AI "auto" belum dikonfigurasi (atau sedang rate limit/tidak bisa dijangkau). Tambahkan API key yang sesuai di Admin -> Pengaturan AI.`,
		})
		return
	}
	if errors.Is(err, ai.ErrNoImageProviderKey) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Tambahkan minimal satu API key penyedia gambar di Admin -> Pengaturan AI (OpenAI atau Gemini) untuk membuat gambar sampul. Key Groq/Claude tidak dipakai untuk pembuatan gambar.",
		})
		return
	}
	if errors.Is(err, ai.ErrMissingAPIKey) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI belum dikonfigurasi. Tambahkan minimal satu key penyedia di Admin -> Pengaturan AI (ai_setting).",
		})
		return
	}
	if errors.Is(err, ai.ErrUnsupportedProvider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Penyedia tidak valid"})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}

// UploadImage handles image upload for post cover image.
func (h *PostHandler) UploadImage(c *gin.Context) {
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar wajib diunggah"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > postImageMaxBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ukuran gambar harus antara 1 byte dan 5 MB"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file yang diunggah"})
		return
	}
	defer src.Close()

	sniff := make([]byte, 512)
	n, err := src.Read(sniff)
	if err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memvalidasi file yang diunggah"})
		return
	}

	mimeType := http.DetectContentType(sniff[:n])
	if _, ok := allowedImageExtension(mimeType); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya gambar jpeg, png, webp, dan gif yang diizinkan"})
		return
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memproses file yang diunggah"})
		return
	}

	raw, err := io.ReadAll(io.LimitReader(src, postImageMaxBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file yang diunggah"})
		return
	}
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar kosong"})
		return
	}

	maxEdge, webpQ := repositories.PostImageStoreLimitsFromDB(h.db)
	enc, err := media.EncodePostUploadWithStoreCap(raw, mimeType, maxEdge, webpQ)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := os.MkdirAll(postImageDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan folder unggahan"})
		return
	}

	slugField := strings.ToLower(strings.TrimSpace(c.PostForm("slug")))
	filename, err := postImageFilename(slugField, enc.Ext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat nama file"})
		return
	}

	// Prefer R2 when configured; on failure fall back to local disk (reload each request).
	if store := storage.LoadR2ObjectStore(c.Request.Context(), h.db, h.cfg); store != nil {
		key := "posts/" + filename
		url, err := store.Put(c.Request.Context(), key, bytes.NewReader(enc.Data), enc.ContentType)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"image": url})
			return
		}
		log.Printf("post upload-image: R2 failed, using local disk: %v", err)
	}

	targetPath := filepath.Join(postImageDir, filename)
	if err := os.WriteFile(targetPath, enc.Data, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file yang diunggah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"image": postImageURLBase + "/" + filename})
}

// Create
// @Summary      Create post
// @Description  Create a new post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreatePostRequest  true  "Post payload"
// @Success      201   {object}  entity.Post
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/posts [post]
func (h *PostHandler) Create(c *gin.Context) {
	requesterID, requesterRole := requesterFromContext(c)
	var req dto.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tags := make([]entity.Tag, 0, len(req.Tags))
	for _, id := range req.Tags {
		tags = append(tags, entity.Tag{ID: id})
	}

	authorID := req.AuthorID
	if requesterRole == "writer" {
		authorID = requesterID
	}
	includeSource := false
	if req.IncludeSource != nil {
		includeSource = *req.IncludeSource
	}

	input := entity.Post{
		Title:             req.Title,
		Slug:              req.Slug,
		Content:           req.Content,
		Status:            req.Status,
		PublishAt:         req.PublishAt,
		Image:             req.Image,
		ImageOrigin:       req.ImageOrigin,
		ImageOriginDetail: req.ImageOriginDetail,
		Source:            req.Source,
		IncludeSource:     includeSource,
		AuthorID:          authorID,
		CategoryID:        req.CategoryID,
		Tags:              tags,
	}
	if req.MetaDescription != nil {
		v := strings.TrimSpace(*req.MetaDescription)
		input.MetaDesc = &v
	}
	if req.MetaKeywords != nil {
		v := strings.TrimSpace(*req.MetaKeywords)
		input.MetaKeywords = &v
	}

	if err := h.svc.Create(&input); err != nil {
		if errors.Is(err, services.ErrPostSlugTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "Slug post sudah digunakan"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membuat post"})
		return
	}
	if h.autoPoster != nil && strings.EqualFold(strings.TrimSpace(input.Status), "published") {
		_ = h.autoPoster.OnPublishedPost(c.Request.Context(), &input)
	}
	h.maybeAutoIndexGSC(&input)
	c.JSON(http.StatusCreated, input)
}

// Update
// @Summary      Update post
// @Description  Update existing post
// @Tags         posts
// @Accept       json
// @Produce      json
// @Param        id    path      int        true  "Post ID"
// @Param        body  body      dto.UpdatePostRequest true "Post payload"
// @Success      200   {object}  entity.Post
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/posts/{id} [put]
func (h *PostHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	requesterID, requesterRole := requesterFromContext(c)

	var req dto.UpdatePostRequest
	existing, err := h.svc.Get(uint(id))
	if err != nil {
		respondNotFound(c, err)
		return
	}
	if requesterRole == "writer" && existing.AuthorID != requesterID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke post ini"})
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data post tidak valid"})
		return
	}

	var tags []entity.Tag
	if req.Tags != nil {
		tags = make([]entity.Tag, 0, len(req.Tags))
		for _, tid := range req.Tags {
			tags = append(tags, entity.Tag{ID: tid})
		}
	}

	authorID := *req.AuthorID
	if requesterRole == "writer" {
		authorID = requesterID
	}
	includeSource := existing.IncludeSource
	if req.IncludeSource != nil {
		includeSource = *req.IncludeSource
	}

	input := entity.Post{
		Title:             *req.Title,
		Slug:              *req.Slug,
		Content:           *req.Content,
		Status:            *req.Status,
		PublishAt:         req.PublishAt,
		Image:             req.Image,
		ImageOrigin:       req.ImageOrigin,
		ImageOriginDetail: req.ImageOriginDetail,
		Source:            req.Source,
		IncludeSource:     includeSource,
		AuthorID:          authorID,
		CategoryID:        *req.CategoryID,
		Tags:              tags,
	}
	if req.MetaDescription != nil {
		v := strings.TrimSpace(*req.MetaDescription)
		input.MetaDesc = &v
	}
	if req.MetaKeywords != nil {
		v := strings.TrimSpace(*req.MetaKeywords)
		input.MetaKeywords = &v
	}

	wasPublished := strings.EqualFold(strings.TrimSpace(existing.Status), "published")
	item, err := h.svc.Update(uint(id), &input)
	if err != nil {
		if errors.Is(err, services.ErrPostSlugTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "Slug post sudah digunakan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui post"})
		return
	}
	if h.autoPoster != nil && item != nil && !wasPublished && strings.EqualFold(strings.TrimSpace(item.Status), "published") {
		_ = h.autoPoster.OnPublishedPost(c.Request.Context(), item)
	}
	if item != nil && strings.EqualFold(strings.TrimSpace(item.Status), "published") {
		h.maybeAutoIndexGSC(item)
	}
	if existing != nil && item != nil {
		oldImage := strings.TrimSpace(derefString(existing.Image))
		newImage := strings.TrimSpace(derefString(item.Image))
		if oldImage != "" && oldImage != newImage {
			removeUploadedFileIfLocal(oldImage, postImageURLBase, postImageDir)
		}
	}
	c.JSON(http.StatusOK, item)
}

func (h *PostHandler) BulkUpdateStatus(c *gin.Context) {
	requesterID, requesterRole := requesterFromContext(c)
	var req struct {
		IDs    []uint `json:"ids"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data status post tidak valid"})
		return
	}
	targetStatus := strings.ToLower(strings.TrimSpace(req.Status))
	wasPublished := make(map[uint]bool, len(req.IDs))
	for _, id := range req.IDs {
		if id == 0 {
			continue
		}
		item, err := h.svc.Get(id)
		if err == nil && item != nil {
			wasPublished[id] = strings.EqualFold(strings.TrimSpace(item.Status), "published")
		}
	}

	updated, err := h.svc.BulkUpdateStatus(req.IDs, req.Status, requesterID, requesterRole)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memperbarui status post"})
		return
	}
	if h.autoPoster != nil && targetStatus == "published" {
		for _, id := range req.IDs {
			if id == 0 || wasPublished[id] {
				continue
			}
			item, getErr := h.svc.Get(id)
			if getErr != nil || item == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(item.Status), "published") {
				_ = h.autoPoster.OnPublishedPost(c.Request.Context(), item)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

// Delete
// @Summary      Delete post
// @Description  Delete post by ID
// @Tags         posts
// @Produce      json
// @Param        id   path      int  true  "Post ID"
// @Success      204  "No Content"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/posts/{id} [delete]
func (h *PostHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	requesterID, requesterRole := requesterFromContext(c)
	existing, err := h.svc.Get(uint(id))
	if err != nil {
		respondNotFound(c, err)
		return
	}
	if requesterRole == "writer" {
		if existing.AuthorID != requesterID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke post ini"})
			return
		}
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus post"})
		return
	}
	if existing != nil {
		removeUploadedFileIfLocal(derefString(existing.Image), postImageURLBase, postImageDir)
	}
	c.Status(http.StatusNoContent)
}

func allowedImageExtension(mimeType string) (string, bool) {
	switch mimeType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

func newPostImageFilename(ext string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%d%s", hex.EncodeToString(b[:]), time.Now().Unix(), ext), nil
}

func postImageFilename(slug, ext string) (string, error) {
	if slug != "" && len(slug) <= postImageSlugLen && postImageSlugRe.MatchString(slug) {
		return fmt.Sprintf("%s-%d%s", slug, time.Now().Unix(), ext), nil
	}
	return newPostImageFilename(ext)
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func removeUploadedFileIfLocal(rawURL, urlBase, dir string) {
	v := strings.TrimSpace(rawURL)
	if v == "" {
		return
	}
	if u, err := url.Parse(v); err == nil && u != nil && u.Scheme != "" && u.Host != "" {
		// Absolute URL: treat as local only when path points to the same upload base.
		v = strings.TrimSpace(u.Path)
	}
	if i := strings.Index(v, "?"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, "#"); i >= 0 {
		v = v[:i]
	}
	base := strings.TrimRight(strings.TrimSpace(urlBase), "/")
	if base == "" {
		return
	}
	if !strings.HasPrefix(v, base+"/") {
		return
	}
	name := strings.TrimPrefix(v, base+"/")
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return
	}
	_ = os.Remove(filepath.Join(dir, name))
}
