package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/ai"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
)

// SiteLayoutHandler serves custom layout CRUD for admin.
type SiteLayoutHandler struct {
	svc       services.SiteLayoutService
	ai        *ai.Generator
	aiSet     repositories.AISettingRepository
	db        *gorm.DB
	cfg       appconfig.Config
	genJobsMu sync.RWMutex
	genJobs   map[string]*layoutGenerateJob
	genQueue  chan *layoutGenerateJob
	genOnce   sync.Once
}

func NewSiteLayoutHandler(svc services.SiteLayoutService, aiGen *ai.Generator, aiSet repositories.AISettingRepository, db *gorm.DB, cfg appconfig.Config) *SiteLayoutHandler {
	return &SiteLayoutHandler{svc: svc, ai: aiGen, aiSet: aiSet, db: db, cfg: cfg}
}

func isDuplicateSiteLayoutErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "duplicate entry") &&
		strings.Contains(msg, "layout_name")
}

func siteLayoutServiceError(err error) (string, int) {
	switch {
	case errors.Is(err, services.ErrSiteLayoutInUse):
		return "Layout sedang digunakan dan tidak dapat dihapus.", http.StatusConflict
	case errors.Is(err, services.ErrSiteLayoutNotFound):
		return "Layout tidak ditemukan.", http.StatusNotFound
	case errors.Is(err, services.ErrSiteLayoutNameTaken):
		return "Nama layout sudah digunakan.", http.StatusConflict
	default:
		return "", 0
	}
}

func siteLayoutFromCreateDTO(req dto.CreateSiteLayoutRequest) entity.SiteLayout {
	return entity.SiteLayout{
		LayoutName:    req.LayoutName,
		Header:        req.Header,
		Footer:        req.Footer,
		HomeBody:      req.HomeBody,
		ArticleBody:   req.ArticleBody,
		Sidebar:       req.Sidebar,
		RSS:           req.RSS,
		AuthorProfile: req.AuthorProfile,
		Tag:           req.Tag,
		Kategori:      req.Kategori,
		SearchPage:    req.SearchPage,
		Misc:          req.Misc,
		LatestPrompt:  req.LatestPrompt,
		CurrentPrompt: req.CurrentPrompt,
		ImageRef:      req.ImageRef,
		CSSData:       req.CSSData,
		JSData:        req.JSData,
	}
}

// List returns paginated site layouts.
func (h *SiteLayoutHandler) List(c *gin.Context) {
	page, size := parsePagination(c)
	q := strings.TrimSpace(c.Query("q"))

	items, total, err := h.svc.List(page, size, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat layout"})
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

// GetByID returns one site layout.
func (h *SiteLayoutHandler) GetByID(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.svc.Get(uint(id))
	if err != nil {
		if msg, code := siteLayoutServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Layout tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Create stores a new site layout.
func (h *SiteLayoutHandler) Create(c *gin.Context) {
	var req dto.CreateSiteLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data layout tidak valid"})
		return
	}

	input := siteLayoutFromCreateDTO(req)
	if err := h.svc.Create(&input); err != nil {
		if isDuplicateSiteLayoutErr(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Nama layout sudah digunakan."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat layout"})
		return
	}
	c.JSON(http.StatusCreated, input)
}

// Update replaces an existing site layout.
func (h *SiteLayoutHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req dto.CreateSiteLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data layout tidak valid"})
		return
	}

	input := siteLayoutFromCreateDTO(req)
	item, err := h.svc.Update(uint(id), &input)
	if err != nil {
		if msg, code := siteLayoutServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		if isDuplicateSiteLayoutErr(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Nama layout sudah digunakan."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui layout"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Duplicate copies a site layout under a new name; is_used is always false on the copy.
func (h *SiteLayoutHandler) Duplicate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req dto.DuplicateSiteLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama layout tidak valid"})
		return
	}

	item, err := h.svc.Duplicate(uint(id), req.LayoutName)
	if err != nil {
		if msg, code := siteLayoutServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		if isDuplicateSiteLayoutErr(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Nama layout sudah digunakan."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menduplikasi layout"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// Delete removes a site layout when it is not in use.
func (h *SiteLayoutHandler) Delete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(uint(id)); err != nil {
		if msg, code := siteLayoutServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus layout"})
		return
	}
	c.Status(http.StatusNoContent)
}
