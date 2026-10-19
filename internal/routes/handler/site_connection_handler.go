package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
	"github.com/Bebii-Digital/bebii-blog/internal/social"
)

// SiteConnectionHandler serves the singleton site_connections resource.
type SiteConnectionHandler struct {
	svc        services.SiteConnectionService
	autoPoster *social.AutoPoster
}

// NewSiteConnectionHandler constructs SiteConnectionHandler.
func NewSiteConnectionHandler(svc services.SiteConnectionService, autoPoster *social.AutoPoster) *SiteConnectionHandler {
	return &SiteConnectionHandler{svc: svc, autoPoster: autoPoster}
}

// Get returns verification fields (empty strings if no row yet).
func (h *SiteConnectionHandler) Get(c *gin.Context) {
	item, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat koneksi site"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Update saves Ahrefs, Google, Bing, and Yandex verification values.
func (h *SiteConnectionHandler) Update(c *gin.Context) {
	var req dto.UpdateSiteConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data koneksi site tidak valid"})
		return
	}

	item, err := h.svc.Update(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan koneksi site"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// TestAutoPost triggers all enabled social webhooks with sample payload.
func (h *SiteConnectionHandler) TestAutoPost(c *gin.Context) {
	if h.autoPoster == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Layanan autopost belum dikonfigurasi"})
		return
	}
	var req dto.TestAutoPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tes autopost tidak valid"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Test autopost " + time.Now().Format("2006-01-02 15:04")
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = "test-autopost"
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		content = "Ini konten percobaan untuk mengetes webhook autopost."
	}
	p := &entity.Post{
		ID:      1,
		Title:   title,
		Slug:    slug,
		Content: content,
		Status:  "published",
		Image:   req.Image,
	}
	if err := h.autoPoster.OnPublishedPost(c.Request.Context(), p); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Tes autopost dikirim ke platform aktif"})
}
