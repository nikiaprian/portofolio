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

func isDuplicateTagErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "duplicate entry") &&
		(strings.Contains(msg, "tags.idx_tags_name") || strings.Contains(msg, "tags.idx_tags_slug"))
}

// TagHandler struct
type TagHandler struct {
	svc services.TagService
	db  *gorm.DB
}

// function to create a new TagHandler
func NewTagHandler(svc services.TagService, db *gorm.DB) *TagHandler {
	return &TagHandler{svc: svc, db: db}
}

func (h *TagHandler) triggerGSCIndexing(tag *entity.Tag, reqCtx *gin.Context) (string, error) {
	if h.db == nil || tag == nil || tag.ID == 0 {
		return "", errors.New("tag tidak valid")
	}
	var sc entity.SiteConnection
	if err := h.db.First(&sc).Error; err != nil {
		return "", errors.New("gagal memuat konfigurasi Site Connection")
	}
	if strings.TrimSpace(sc.GoogleServiceAccountJSON) == "" {
		return "", errors.New("kredensial Google Service Account JSON belum dikonfigurasi")
	}

	tagURL := postutil.BuildPublicTagURL(h.db, tag, seo.PublicSiteOrigin(reqCtx))
	now := time.Now().UTC()

	inspect, inspectErr := gsc.InspectURL(sc.GoogleServiceAccountJSON, "", tagURL)
	if inspectErr == nil && inspect != nil && inspect.IsIndexed {
		msg := "Terindeks di Google"
		if inspect.CoverageState != "" {
			msg = fmt.Sprintf("Terindeks di Google (%s)", inspect.CoverageState)
		}
		_ = h.db.Model(&entity.Tag{}).Where("id = ?", tag.ID).Updates(map[string]interface{}{
			"indexing_status":  entity.IndexingStatusIndexed,
			"indexed_at":       now,
			"indexing_message": msg,
		}).Error
		return tagURL, nil
	}

	resp, err := gsc.RequestURLIndexing(sc.GoogleServiceAccountJSON, tagURL, "URL_UPDATED")
	if err != nil {
		msg := err.Error()
		if inspect != nil && inspect.CoverageState != "" {
			msg = fmt.Sprintf("%s (%s)", inspect.CoverageState, err.Error())
		}
		if len(msg) > 490 {
			msg = msg[:490]
		}
		_ = h.db.Model(&entity.Tag{}).Where("id = ?", tag.ID).Updates(map[string]interface{}{
			"indexing_status":  entity.IndexingStatusFailed,
			"indexed_at":       now,
			"indexing_message": msg,
		}).Error
		return tagURL, err
	}

	msg := "Terkirim ke Google Indexing API"
	if resp != nil && resp.URLNotificationMetadata.LatestUpdate.NotifyTime != "" {
		msg = fmt.Sprintf("Terkirim: %s", resp.URLNotificationMetadata.LatestUpdate.NotifyTime)
	}

	_ = h.db.Model(&entity.Tag{}).Where("id = ?", tag.ID).Updates(map[string]interface{}{
		"indexing_status":  entity.IndexingStatusSubmitted,
		"indexed_at":       now,
		"indexing_message": msg,
	}).Error

	return tagURL, nil
}

func (h *TagHandler) maybeAutoIndexGSC(t *entity.Tag) {
	if h.db == nil || t == nil || t.ID == 0 {
		return
	}
	var sc entity.SiteConnection
	if err := h.db.First(&sc).Error; err != nil || !sc.GoogleIndexingEnabled || strings.TrimSpace(sc.GoogleServiceAccountJSON) == "" {
		return
	}
	go func(tagID uint) {
		var current entity.Tag
		if err := h.db.First(&current, tagID).Error; err != nil {
			return
		}
		_, _ = h.triggerGSCIndexing(&current, nil)
	}(t.ID)
}

func (h *TagHandler) IndexGSC(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID tag tidak valid"})
		return
	}
	tag, err := h.svc.Get(uint(id64))
	if err != nil || tag == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag tidak ditemukan"})
		return
	}

	tagURL, err := h.triggerGSCIndexing(tag, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pengindeksan Google Search Console berhasil dikirim",
		"tag_url": tagURL,
	})
}

func (h *TagHandler) BulkIndexGSC(c *gin.Context) {
	var body struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Daftar ID tag tidak valid"})
		return
	}

	var tags []entity.Tag
	if err := h.db.Where("id IN ?", body.IDs).Find(&tags).Error; err != nil || len(tags) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag tidak ditemukan"})
		return
	}

	submitted := 0
	var lastErr error
	for _, tag := range tags {
		item := tag
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
		"message":   fmt.Sprintf("%d dari %d tag berhasil dikirim ke Google Indexing API", submitted, len(tags)),
		"submitted": submitted,
		"total":     len(tags),
	})
}

// List
// @Summary      List tags
// @Description  Get tags with pagination
// @Tags         tags
// @Produce      json
// @Param        page  query     int  false  "Page number" default(1)
// @Param        size  query     int  false  "Page size (max 500)" default(10)
// @Success      200  {object}  dto.PaginatedTagsResponse
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/tags [get]
func (h *TagHandler) List(c *gin.Context) {
	page, size := parsePagination(c)
	q := strings.TrimSpace(c.Query("q"))

	// list tags with pagination
	items, total, err := h.svc.List(page, size, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat tag"})
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
// @Summary      Get tag by ID
// @Description  Get a single tag by ID
// @Tags         tags
// @Produce      json
// @Param        id   path      int  true  "Tag ID"
// @Success      200  {object}  entity.Tag
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/tags/{id} [get]
func (h *TagHandler) GetByID(c *gin.Context) {
	// get the tag by ID
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// GetPublicBySlug returns a public-safe tag for client pages.
// @Summary      Get tag by slug (public)
// @Description  Get a single tag by slug for client/headless frontends
// @Tags         tags
// @Produce      json
// @Param        slug  path      string  true  "Tag slug"
// @Success      200   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]string
// @Router       /tag/{slug} [get]
func (h *TagHandler) GetPublicBySlug(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	item, err := h.svc.GetBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag tidak ditemukan"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":   item.ID,
		"name": item.Name,
		"slug": item.Slug,
	})
}

// Create
// @Summary      Create tag
// @Description  Create a new tag
// @Tags         tags
// @Accept       json
// @Produce      json
// @Param        body  body      dto.CreateTagRequest  true  "Tag payload"
// @Success      201   {object}  entity.Tag
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/tags [post]
func (h *TagHandler) Create(c *gin.Context) {
	var req dto.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tag tidak valid"})
		return
	}

	input := entity.Tag{
		Name: req.Name,
		Slug: req.Slug,
	}
	if err := h.svc.Create(&input); err != nil {
		if isDuplicateTagErr(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Nama atau slug tag sudah digunakan."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat tag"})
		return
	}
	h.maybeAutoIndexGSC(&input)
	c.JSON(http.StatusCreated, input)
}

// Update
// @Summary      Update tag
// @Description  Update existing tag
// @Tags         tags
// @Accept       json
// @Produce      json
// @Param        id    path      int      true  "Tag ID"
// @Param        body  body      dto.UpdateTagRequest true "Tag payload"
// @Success      200   {object}  entity.Tag
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/tags/{id} [put]
func (h *TagHandler) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req dto.UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tag tidak valid"})
		return
	}

	input := entity.Tag{
		Name: *req.Name,
		Slug: *req.Slug,
	}
	item, err := h.svc.Update(uint(id), &input)
	if err != nil {
		if isDuplicateTagErr(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Nama atau slug tag sudah digunakan."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui tag"})
		return
	}
	h.maybeAutoIndexGSC(item)
	c.JSON(http.StatusOK, item)
}

// Delete
// @Summary      Delete tag
// @Description  Delete tag by ID
// @Tags         tags
// @Produce      json
// @Param        id   path      int  true  "Tag ID"
// @Success      204  "No Content"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/tags/{id} [delete]
func (h *TagHandler) Delete(c *gin.Context) {
	// delete the tag
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus tag"})
		return
	}
	c.Status(http.StatusNoContent)
}
