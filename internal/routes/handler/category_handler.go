package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/gsc"
	postutil "github.com/Bebii-Digital/bebii-blog/internal/helper/post"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
)

// CategoryHandler struct
type CategoryHandler struct {
	svc services.CategoryService
	db  *gorm.DB
}

// function to create a new CategoryHandler
func NewCategoryHandler(svc services.CategoryService, db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{svc: svc, db: db}
}

func (h *CategoryHandler) triggerGSCIndexing(cat *entity.Category, reqCtx *gin.Context) (string, error) {
	if h.db == nil || cat == nil || cat.ID == 0 {
		return "", errors.New("kategori tidak valid")
	}
	var sc entity.SiteConnection
	if err := h.db.First(&sc).Error; err != nil {
		return "", errors.New("gagal memuat konfigurasi Site Connection")
	}
	if strings.TrimSpace(sc.GoogleServiceAccountJSON) == "" {
		return "", errors.New("kredensial Google Service Account JSON belum dikonfigurasi")
	}

	catURL := postutil.BuildPublicCategoryURL(h.db, cat, seo.PublicSiteOrigin(reqCtx))
	now := time.Now().UTC()

	inspect, inspectErr := gsc.InspectURL(sc.GoogleServiceAccountJSON, "", catURL)
	if inspectErr == nil && inspect != nil && inspect.IsIndexed {
		msg := "Terindeks di Google"
		if inspect.CoverageState != "" {
			msg = fmt.Sprintf("Terindeks di Google (%s)", inspect.CoverageState)
		}
		_ = h.db.Model(&entity.Category{}).Where("id = ?", cat.ID).Updates(map[string]interface{}{
			"indexing_status":  entity.IndexingStatusIndexed,
			"indexed_at":       now,
			"indexing_message": msg,
		}).Error
		return catURL, nil
	}

	resp, err := gsc.RequestURLIndexing(sc.GoogleServiceAccountJSON, catURL, "URL_UPDATED")
	if err != nil {
		msg := err.Error()
		if inspect != nil && inspect.CoverageState != "" {
			msg = fmt.Sprintf("%s (%s)", inspect.CoverageState, err.Error())
		}
		if len(msg) > 490 {
			msg = msg[:490]
		}
		_ = h.db.Model(&entity.Category{}).Where("id = ?", cat.ID).Updates(map[string]interface{}{
			"indexing_status":  entity.IndexingStatusFailed,
			"indexed_at":       now,
			"indexing_message": msg,
		}).Error
		return catURL, err
	}

	msg := "Terkirim ke Google Indexing API"
	if resp != nil && resp.URLNotificationMetadata.LatestUpdate.NotifyTime != "" {
		msg = fmt.Sprintf("Terkirim: %s", resp.URLNotificationMetadata.LatestUpdate.NotifyTime)
	}

	_ = h.db.Model(&entity.Category{}).Where("id = ?", cat.ID).Updates(map[string]interface{}{
		"indexing_status":  entity.IndexingStatusSubmitted,
		"indexed_at":       now,
		"indexing_message": msg,
	}).Error

	return catURL, nil
}

func (h *CategoryHandler) maybeAutoIndexGSC(cat *entity.Category) {
	if h.db == nil || cat == nil || cat.ID == 0 {
		return
	}
	var sc entity.SiteConnection
	if err := h.db.First(&sc).Error; err != nil || !sc.GoogleIndexingEnabled || strings.TrimSpace(sc.GoogleServiceAccountJSON) == "" {
		return
	}
	go func(catID uint) {
		var current entity.Category
		if err := h.db.Preload("Parent").First(&current, catID).Error; err != nil {
			return
		}
		_, _ = h.triggerGSCIndexing(&current, nil)
	}(cat.ID)
}

func (h *CategoryHandler) IndexGSC(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kategori tidak valid"})
		return
	}
	cat, err := h.svc.Get(uint(id64))
	if err != nil || cat == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
		return
	}

	catURL, err := h.triggerGSCIndexing(cat, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Pengindeksan Google Search Console berhasil dikirim",
		"category_url": catURL,
	})
}

func (h *CategoryHandler) BulkIndexGSC(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Daftar ID kategori tidak valid"})
		return
	}

	var categories []entity.Category
	if err := h.db.Preload("Parent").Where("id IN ?", body.IDs).Find(&categories).Error; err != nil || len(categories) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
		return
	}

	submitted := 0
	var lastErr error
	for _, cat := range categories {
		item := cat
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
		"message":   fmt.Sprintf("%d dari %d kategori berhasil dikirim ke Google Indexing API", submitted, len(categories)),
		"submitted": submitted,
		"total":     len(categories),
	})
}

// List
// @Summary      List categories
// @Description  Get categories with pagination
// @Tags         categories
// @Produce      json
// @Param        page  query     int  false  "Page number" default(1)
// @Param        size  query     int  false  "Page size (max 500)" default(10)
// @Success      200  {object}  dto.PaginatedCategoriesResponse
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	page, size := parsePagination(c)
	q := strings.TrimSpace(c.Query("q"))

	onlyEnabled := !strings.Contains(c.Request.URL.Path, "/admin")

	// list categories with pagination
	items, total, err := h.svc.List(page, size, q, onlyEnabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat kategori"})
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
// @Summary      Get category by ID
// @Description  Get a single category by ID
// @Tags         categories
// @Produce      json
// @Param        id   path      int  true  "Category ID"
// @Success      200  {object}  entity.Category
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/categories/{id} [get]
func (h *CategoryHandler) GetByID(c *gin.Context) {
	// get the category by ID
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Create
// @Summary      Create category
// @Description  Create a new category
// @Tags         categories
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateCategoryRequest  true  "Category payload"
// @Success      201   {object}  entity.Category
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data kategori tidak valid"})
		return
	}

	input := entity.Category{
		Name:        req.Name,
		Slug:        req.Slug,
		Title:       strings.TrimSpace(req.Title),
		Keyword:     strings.TrimSpace(req.Keyword),
		Description: strings.TrimSpace(req.Description),
		ParentID:    req.ParentID,
	}
	if req.Sort != nil {
		input.Sort = *req.Sort
	}
	if req.Enabled != nil {
		input.Enabled = *req.Enabled
	}
	if input.Title == "" {
		input.Title = strings.TrimSpace(req.Name)
	}

	if err := h.svc.Create(&input); err != nil {
		if msg, code := categoryServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat kategori"})
		return
	}
	h.maybeAutoIndexGSC(&input)
	c.JSON(http.StatusCreated, input)
}

// Update
// @Summary      Update category
// @Description  Update existing category
// @Tags         categories
// @Accept       json
// @Produce      json
// @Param        id    path      int            true  "Category ID"
// @Param        body  body      dto.UpdateCategoryRequest true "Category payload"
// @Success      200   {object}  entity.Category
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/categories/{id} [put]
func (h *CategoryHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data kategori tidak valid"})
		return
	}

	existing, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kategori tidak ditemukan"})
		return
	}
	existing.Name = *req.Name
	existing.Slug = *req.Slug
	if req.Sort != nil {
		existing.Sort = *req.Sort
	}
	if req.Title != nil {
		existing.Title = strings.TrimSpace(*req.Title)
	}
	if strings.TrimSpace(existing.Title) == "" {
		existing.Title = strings.TrimSpace(*req.Name)
	}
	if req.Keyword != nil {
		existing.Keyword = strings.TrimSpace(*req.Keyword)
	}
	if req.Description != nil {
		existing.Description = strings.TrimSpace(*req.Description)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.ClearParent != nil && *req.ClearParent {
		existing.ParentID = nil
	} else if req.ParentID != nil {
		existing.ParentID = req.ParentID
	}

	item, err := h.svc.Update(uint(id), existing)
	if err != nil {
		if msg, code := categoryServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui kategori"})
		return
	}
	h.maybeAutoIndexGSC(item)
	c.JSON(http.StatusOK, item)
}

// Delete
// @Summary      Delete category
// @Description  Delete category by ID
// @Tags         categories
// @Produce      json
// @Param        id   path      int  true  "Category ID"
// @Success      204  "No Content"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	// delete the category
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(uint(id)); err != nil {
		if msg, code := categoryServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus kategori"})
		return
	}
	c.Status(http.StatusNoContent)
}

// BulkUpdateStatus enables or disables multiple categories at once.
func (h *CategoryHandler) BulkUpdateStatus(c *gin.Context) {
	var req struct {
		IDs     []uint `json:"ids"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tidak valid"})
		return
	}
	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tidak ada kategori dipilih"})
		return
	}
	updated := int64(0)
	for _, id := range req.IDs {
		if id == 0 {
			continue
		}
		cat, err := h.svc.Get(id)
		if err != nil {
			continue
		}
		cat.Enabled = req.Enabled
		if _, err := h.svc.Update(id, cat); err == nil {
			updated++
		}
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

func categoryServiceError(err error) (string, int) {
	switch {
	case errors.Is(err, services.ErrCategoryParentNotFound):
		return "Kategori induk tidak ditemukan", http.StatusBadRequest
	case errors.Is(err, services.ErrCategoryInvalidParent):
		return "Kategori induk tidak valid", http.StatusBadRequest
	case errors.Is(err, services.ErrCategoryMaxDepth):
		return "Subkategori tidak boleh memiliki subkategori lagi", http.StatusBadRequest
	case errors.Is(err, services.ErrCategoryHasChildren):
		return "Hapus subkategori terlebih dahulu sebelum menghapus atau memindahkan kategori induk ini", http.StatusBadRequest
	default:
		return "", 0
	}
}
