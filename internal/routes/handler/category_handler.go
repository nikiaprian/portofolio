package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
	"github.com/nikiaprian/tirtayasa/internal/services"
)

// CategoryHandler struct
type CategoryHandler struct {
	svc services.CategoryService
}

// function to create a new CategoryHandler
func NewCategoryHandler(svc services.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
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

	// list categories with pagination
	items, total, err := h.svc.List(page, size, q)
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
