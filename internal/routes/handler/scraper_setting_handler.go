package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/ai"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/scraper"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
)

// ScraperSettingHandler serves scraper settings resource.
type ScraperSettingHandler struct {
	svc   services.ScraperSettingService
	db    *gorm.DB
	aiGen *ai.Generator
	cfg   appconfig.Config
}

// NewScraperSettingHandler constructs ScraperSettingHandler.
func NewScraperSettingHandler(
	svc services.ScraperSettingService,
	db *gorm.DB,
	aiGen *ai.Generator,
	cfg appconfig.Config,
) *ScraperSettingHandler {
	return &ScraperSettingHandler{svc: svc, db: db, aiGen: aiGen, cfg: cfg}
}

// Get
// @Summary      Get news scraper settings
// @Description  Returns master switch, interval, and per-category RSS/Atom feeds (one entry per category, each with its own author user email).
// @Tags         scraper-setting
// @Produce      json
// @Success      200  {object}  dto.ScraperSettingResponse
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/scraper-setting [get]
func (h *ScraperSettingHandler) Get(c *gin.Context) {
	item, err := h.svc.Get()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "scraper settings not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Update
// @Summary      Update news scraper settings
// @Description  Updates master settings. If category_feeds is sent, replaces stored feeds per category (include every category; use enabled to toggle scraping per slug). AI paraphrase prompts live in ai_setting, not here.
// @Tags         scraper-setting
// @Accept       json
// @Produce      json
// @Param        body  body      dto.UpdateScraperSettingRequest  true  "Scraper settings"
// @Success      200   {object}  dto.ScraperSettingResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/scraper-setting [put]
func (h *ScraperSettingHandler) Update(c *gin.Context) {
	var req dto.UpdateScraperSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.Update(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// RunNow
// @Summary      Run scraper now
// @Description  Triggers one immediate scraper cycle in background (without waiting interval)
// @Tags         scraper-setting
// @Produce      json
// @Success      202  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/scraper-setting/run-now [post]
func (h *ScraperSettingHandler) RunNow(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database tidak tersedia"})
		return
	}
	ok, err := scraper.HasRunnableJobs(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log scraper"})
		return
	}
	if !ok {
		c.JSON(http.StatusOK, gin.H{"message": "Scraper dilewati: tidak ada kategori aktif dengan URL feed"})
		return
	}
	go scraper.RunOnceNow(h.db, h.aiGen, h.cfg)
	c.JSON(http.StatusAccepted, gin.H{"message": "Scraper mulai berjalan"})
}

// ListLogs returns recent scraper log rows (newest first). Older rows are removed on a rolling window shared with login and bulk post logs (ADMIN_LOG_RETENTION_HOURS).
// @Summary      List scraper logs
// @Description  Paginated scraper_logs for admin review
// @Tags         scraper-setting
// @Produce      json
// @Param        page   query     int     false  "Page (1-based)"
// @Param        size   query     int     false  "Page size (max 500)"
// @Param        level  query     string  false  "Filter by level (info, warn, error)"
// @Param        q      query     string  false  "Search message, feed URL, or category slug"
// @Success      200    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/scraper-logs [get]
func (h *ScraperSettingHandler) ListLogs(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database tidak tersedia"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 500 {
		size = 500
	}
	offset := (page - 1) * size

	lv := strings.ToLower(strings.TrimSpace(c.Query("level")))
	base := h.db.Model(&entity.ScraperLog{})
	if lv != "" {
		base = base.Where("level = ?", lv)
	}
	if pat, ok := logSearchLike(c.Query("q")); ok {
		base = base.Where(
			"(message LIKE ? OR feed_url LIKE ? OR category_slug LIKE ? OR provider_used LIKE ?)",
			pat, pat, pat, pat,
		)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log scraper"})
		return
	}
	var items []entity.ScraperLog
	q2 := h.db.Model(&entity.ScraperLog{})
	if lv != "" {
		q2 = q2.Where("level = ?", lv)
	}
	if pat, ok := logSearchLike(c.Query("q")); ok {
		q2 = q2.Where(
			"(message LIKE ? OR feed_url LIKE ? OR category_slug LIKE ? OR provider_used LIKE ?)",
			pat, pat, pat, pat,
		)
	}
	if err := q2.Order("id DESC").Limit(size).Offset(offset).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		pu, msg := AugmentInlineLogProviders(it.Message, it.ProviderUsed)
		out = append(out, gin.H{
			"id":            it.ID,
			"level":         it.Level,
			"message":       msg,
			"category_slug": it.CategorySlug,
			"feed_url":      it.FeedURL,
			"provider_used": pu,
			"created_at":    it.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// ClearLogs deletes all scraper log rows. Admin-only route.
func (h *ScraperSettingHandler) ClearLogs(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database tidak tersedia"})
		return
	}
	res := h.db.Where("1 = 1").Delete(&entity.ScraperLog{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membersihkan log scraper"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": res.RowsAffected})
}
