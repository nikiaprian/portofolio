package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/gsc"
	postutil "github.com/Bebii-Digital/bebii-blog/internal/helper/post"
	posthtml "github.com/Bebii-Digital/bebii-blog/internal/helper/post/html"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
	"github.com/Bebii-Digital/bebii-blog/internal/media"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
)

// PageHandler struct
type PageHandler struct {
	svc services.PageService
	db  *gorm.DB
	cfg appconfig.Config
}

// function to create a new PageHandler
func NewPageHandler(svc services.PageService, db *gorm.DB, cfg appconfig.Config) *PageHandler {
	return &PageHandler{svc: svc, db: db, cfg: cfg}
}

func (h *PageHandler) triggerGSCIndexing(p *entity.Page, reqCtx *gin.Context) (string, error) {
	if h.db == nil || p == nil || p.ID == 0 {
		return "", errors.New("halaman tidak valid")
	}
	var sc entity.SiteConnection
	if err := h.db.First(&sc).Error; err != nil {
		return "", errors.New("gagal memuat konfigurasi Site Connection")
	}
	if strings.TrimSpace(sc.GoogleServiceAccountJSON) == "" {
		return "", errors.New("kredensial Google Service Account JSON belum dikonfigurasi")
	}

	pageURL := postutil.BuildPublicPageURL(h.db, p, seo.PublicSiteOrigin(reqCtx))
	now := time.Now().UTC()

	inspect, inspectErr := gsc.InspectURL(sc.GoogleServiceAccountJSON, "", pageURL)
	if inspectErr == nil && inspect != nil && inspect.IsIndexed {
		msg := "Terindeks di Google"
		if inspect.CoverageState != "" {
			msg = fmt.Sprintf("Terindeks di Google (%s)", inspect.CoverageState)
		}
		_ = h.db.Model(&entity.Page{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
			"indexing_status":  entity.IndexingStatusIndexed,
			"indexed_at":       now,
			"indexing_message": msg,
		}).Error
		return pageURL, nil
	}

	resp, err := gsc.RequestURLIndexing(sc.GoogleServiceAccountJSON, pageURL, "URL_UPDATED")
	if err != nil {
		msg := err.Error()
		if inspect != nil && inspect.CoverageState != "" {
			msg = fmt.Sprintf("%s (%s)", inspect.CoverageState, err.Error())
		}
		if len(msg) > 490 {
			msg = msg[:490]
		}
		_ = h.db.Model(&entity.Page{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
			"indexing_status":  entity.IndexingStatusFailed,
			"indexed_at":       now,
			"indexing_message": msg,
		}).Error
		return pageURL, err
	}

	msg := "Terkirim ke Google Indexing API"
	if resp != nil && resp.URLNotificationMetadata.LatestUpdate.NotifyTime != "" {
		msg = fmt.Sprintf("Terkirim: %s", resp.URLNotificationMetadata.LatestUpdate.NotifyTime)
	}

	_ = h.db.Model(&entity.Page{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
		"indexing_status":  entity.IndexingStatusSubmitted,
		"indexed_at":       now,
		"indexing_message": msg,
	}).Error

	return pageURL, nil
}

func (h *PageHandler) maybeAutoIndexGSC(p *entity.Page) {
	if h.db == nil || p == nil || p.ID == 0 {
		return
	}
	var sc entity.SiteConnection
	if err := h.db.First(&sc).Error; err != nil || !sc.GoogleIndexingEnabled || strings.TrimSpace(sc.GoogleServiceAccountJSON) == "" {
		return
	}
	go func(pageID uint) {
		var current entity.Page
		if err := h.db.First(&current, pageID).Error; err != nil {
			return
		}
		_, _ = h.triggerGSCIndexing(&current, nil)
	}(p.ID)
}

func (h *PageHandler) IndexGSC(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID halaman tidak valid"})
		return
	}
	p, err := h.svc.Get(uint(id64))
	if err != nil || p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Halaman tidak ditemukan"})
		return
	}

	pageURL, err := h.triggerGSCIndexing(p, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Pengindeksan Google Search Console berhasil dikirim",
		"page_url": pageURL,
	})
}

func (h *PageHandler) BulkIndexGSC(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Daftar ID halaman tidak valid"})
		return
	}

	var pages []entity.Page
	if err := h.db.Where("id IN ?", body.IDs).Find(&pages).Error; err != nil || len(pages) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Halaman tidak ditemukan"})
		return
	}

	submitted := 0
	var lastErr error
	for _, p := range pages {
		item := p
		_, err := h.triggerGSCIndexing(&item, c)
		if err == nil {
			submitted++
		} else {
			lastErr = err
		}
	}

	if submitted == 0 && lastErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": lastErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("%d dari %d halaman berhasil dikirim ke Google Indexing API", submitted, len(pages)),
		"submitted": submitted,
		"total":     len(pages),
	})
}

const (
	pageImageMaxBytes = 5 * 1024 * 1024
	pageImageDir      = "public/static/uploads/pages"
	pageImageURLBase  = "/static/uploads/pages"
)

// List
// @Summary      List pages
// @Description  Get pages with pagination
// @Tags         pages
// @Produce      json
// @Param        page  query     int  false  "Page number" default(1)
// @Param        size  query     int  false  "Page size (max 500)" default(10)
// @Success      200  {object}  dto.PaginatedPagesResponse
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/pages [get]
func (h *PageHandler) List(c *gin.Context) {
	page, size := parsePagination(c)
	q := strings.TrimSpace(c.Query("q"))

	// list pages with pagination
	items, total, err := h.svc.List(page, size, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat halaman"})
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

// GetByID
// @Summary      Get page by ID
// @Description  Get a single page by ID
// @Tags         pages
// @Produce      json
// @Param        id   path      int  true  "Page ID"
// @Success      200  {object}  entity.Page
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/pages/{id} [get]
func (h *PageHandler) GetByID(c *gin.Context) {
	// get the page by ID
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Halaman tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Create
// @Summary      Create page
// @Description  Create a new page
// @Tags         pages
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreatePageRequest  true  "Page payload"
// @Success      201   {object}  entity.Page
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/pages [post]
func (h *PageHandler) Create(c *gin.Context) {
	var req dto.CreatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data halaman tidak valid"})
		return
	}

	input := entity.Page{
		Title:   req.Title,
		Slug:    req.Slug,
		Content: posthtml.SanitizePublicArticleHTML(req.Content),
		Status:  req.Status,
	}

	if err := h.svc.Create(&input); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat halaman"})
		return
	}
	if strings.EqualFold(strings.TrimSpace(input.Status), "published") {
		h.maybeAutoIndexGSC(&input)
	}
	c.JSON(http.StatusCreated, input)
}

// Update
// @Summary      Update page
// @Description  Update existing page
// @Tags         pages
// @Accept       json
// @Produce      json
// @Param        id    path      int            true  "Page ID"
// @Param        body  body      dto.UpdatePageRequest true "Page payload"
// @Success      200   {object}  entity.Page
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/pages/{id} [put]
func (h *PageHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req dto.UpdatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data halaman tidak valid"})
		return
	}

	input := entity.Page{
		Title:   *req.Title,
		Slug:    *req.Slug,
		Content: posthtml.SanitizePublicArticleHTML(*req.Content),
		Status:  *req.Status,
	}

	item, err := h.svc.Update(uint(id), &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui halaman"})
		return
	}
	if item != nil && strings.EqualFold(strings.TrimSpace(item.Status), "published") {
		h.maybeAutoIndexGSC(item)
	}
	c.JSON(http.StatusOK, item)
}

func (h *PageHandler) BulkUpdateStatus(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids"`
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data halaman tidak valid"})
		return
	}
	updated, err := h.svc.BulkUpdateStatus(req.IDs, req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memperbarui status halaman"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

// Delete
// @Summary      Delete page
// @Description  Delete page by ID
// @Tags         pages
// @Produce      json
// @Param        id   path      int  true  "Page ID"
// @Success      204  "No Content"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/pages/{id} [delete]
func (h *PageHandler) Delete(c *gin.Context) {
	// delete the page
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus halaman"})
		return
	}
	c.Status(http.StatusNoContent)
}

// UploadImage handles image upload for page editor content.
func (h *PageHandler) UploadImage(c *gin.Context) {
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar wajib diunggah"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > pageImageMaxBytes {
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
	if _, ok := allowedPageImageExtension(mimeType); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Hanya gambar jpeg, png, webp, dan gif yang diizinkan"})
		return
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memproses file yang diunggah"})
		return
	}

	raw, err := io.ReadAll(io.LimitReader(src, pageImageMaxBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal membaca file yang diunggah"})
		return
	}
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File gambar kosong"})
		return
	}

	enc, err := media.EncodePostUpload(raw, mimeType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal memproses gambar"})
		return
	}

	filename, err := newPageImageFilename(enc.Ext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat nama file"})
		return
	}

	if store := storage.LoadR2ObjectStore(c.Request.Context(), h.db, h.cfg); store != nil {
		key := "pages/" + filename
		url, err := store.Put(c.Request.Context(), key, bytes.NewReader(enc.Data), enc.ContentType)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"image": url})
			return
		}
		log.Printf("page upload-image: R2 failed, using local disk: %v", err)
	}

	if err := os.MkdirAll(pageImageDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyiapkan folder unggahan"})
		return
	}
	targetPath := filepath.Join(pageImageDir, filename)
	if err := os.WriteFile(targetPath, enc.Data, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file yang diunggah"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"image": pageImageURLBase + "/" + filename})
}

func allowedPageImageExtension(mimeType string) (string, bool) {
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

func newPageImageFilename(ext string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "page-" + hex.EncodeToString(b[:]) + ext, nil
}

// GetPublicBySlug returns a single published page by slug (public client API; no auth).
func (h *PageHandler) GetPublicBySlug(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	item, err := h.svc.GetPublishedBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}
