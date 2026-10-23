package handler

import (
	"net/http"

	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
	"github.com/nikiaprian/tirtayasa/internal/services"
	"github.com/gin-gonic/gin"
)

type SiteWebImageProviderHandler struct {
	svc services.SiteWebImageProviderService
}

func NewSiteWebImageProviderHandler(svc services.SiteWebImageProviderService) *SiteWebImageProviderHandler {
	return &SiteWebImageProviderHandler{svc: svc}
}

func (h *SiteWebImageProviderHandler) List(c *gin.Context) {
	items, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat pengaturan API gambar web"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"providers": items})
}

func (h *SiteWebImageProviderHandler) Update(c *gin.Context) {
	var req dto.UpdateSiteWebImageProvidersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data API gambar web tidak valid"})
		return
	}

	items, err := h.svc.Update(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal menyimpan pengaturan API gambar web"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"providers": items})
}
