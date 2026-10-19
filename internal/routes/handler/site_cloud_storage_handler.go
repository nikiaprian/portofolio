package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
)

// SiteCloudStorageHandler serves the singleton site_cloud_storage resource.
type SiteCloudStorageHandler struct {
	svc services.SiteCloudStorageService
}

// NewSiteCloudStorageHandler constructs SiteCloudStorageHandler.
func NewSiteCloudStorageHandler(svc services.SiteCloudStorageService) *SiteCloudStorageHandler {
	return &SiteCloudStorageHandler{svc: svc}
}

// Get returns cloud storage settings (defaults if no row yet).
func (h *SiteCloudStorageHandler) Get(c *gin.Context) {
	item, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat pengaturan Cloudflare R2"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Update saves Cloudflare R2 settings.
func (h *SiteCloudStorageHandler) Update(c *gin.Context) {
	var req dto.UpdateSiteCloudStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data Cloudflare R2 tidak valid"})
		return
	}

	item, err := h.svc.Update(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan pengaturan Cloudflare R2"})
		return
	}
	c.JSON(http.StatusOK, item)
}
