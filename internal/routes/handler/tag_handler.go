package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
	"github.com/nikiaprian/tirtayasa/internal/services"
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
}

// function to create a new TagHandler
func NewTagHandler(svc services.TagService) *TagHandler {
	return &TagHandler{svc: svc}
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
