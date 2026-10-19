package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/alsoread"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/daterecycle"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
)

// SiteSettingHandler serves the singleton site settings resource.
type SiteSettingHandler struct {
	svc services.SiteSettingService
	db  *gorm.DB
	cfg appconfig.Config
}

// NewSiteSettingHandler constructs SiteSettingHandler.
func NewSiteSettingHandler(svc services.SiteSettingService, db *gorm.DB, cfg appconfig.Config) *SiteSettingHandler {
	return &SiteSettingHandler{svc: svc, db: db, cfg: cfg}
}

// Get
// @Summary      Get site settings
// @Description  Returns the current site branding/settings row
// @Tags         site-setting
// @Produce      json
// @Success      200  {object}  entity.SiteSetting
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/site-setting [get]
func (h *SiteSettingHandler) Get(c *gin.Context) {
	item, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Update
// @Summary      Update site settings
// @Description  Updates branding and URLs for the site (singleton)
// @Tags         site-setting
// @Accept       json
// @Produce      json
// @Param        body  body      dto.UpdateSiteSettingRequest  true  "Settings"
// @Success      200   {object}  entity.SiteSetting
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/site-setting [put]
func (h *SiteSettingHandler) Update(c *gin.Context) {
	before, _ := h.svc.Get()
	wasAlsoReadEnabled := before == nil || before.AlsoReadEnabled
	wasAlsoReadMax := entity.DefaultAlsoReadMaxInserts
	wasAlsoReadInterval := entity.DefaultAlsoReadParagraphInterval
	if before != nil {
		wasAlsoReadMax = entity.EffectiveAlsoReadMaxInserts(before.AlsoReadMaxInserts)
		wasAlsoReadInterval = entity.EffectiveAlsoReadParagraphInterval(before.AlsoReadParagraphInterval)
	}

	var req dto.UpdateSiteSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data pengaturan site tidak valid"})
		return
	}

	item, err := h.svc.Update(&req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidSiteScript) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Script site tidak valid"})
			return
		}
		if errors.Is(err, services.ErrInvalidArticleSlugMode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mode URL artikel tidak valid. Pilih: Langsung, Dengan kategori, atau Dengan subkategori."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan pengaturan site"})
		return
	}
	nowAlsoReadMax := entity.DefaultAlsoReadMaxInserts
	nowAlsoReadInterval := entity.DefaultAlsoReadParagraphInterval
	if item != nil {
		nowAlsoReadMax = entity.EffectiveAlsoReadMaxInserts(item.AlsoReadMaxInserts)
		nowAlsoReadInterval = entity.EffectiveAlsoReadParagraphInterval(item.AlsoReadParagraphInterval)
	}
	alsoReadChanged := wasAlsoReadEnabled != (item != nil && item.AlsoReadEnabled) ||
		(item != nil && item.AlsoReadEnabled && (wasAlsoReadMax != nowAlsoReadMax || wasAlsoReadInterval != nowAlsoReadInterval))
	if item != nil && alsoReadChanged && h.db != nil {
		_ = alsoread.SyncPublishedContent(h.db, item.AlsoReadEnabled)
	}
	if before != nil && item != nil {
		removeSiteAssetIfChanged(before.LogoURL, item.LogoURL)
		removeSiteAssetIfChanged(before.FaviconURL, item.FaviconURL)
		removeSiteAssetIfChanged(before.DefaultImage, item.DefaultImage)
	}
	c.JSON(http.StatusOK, item)
}

// UpdateScripts
// @Summary      Update site scripts
// @Description  Updates header/footer scripts and ad slot snippets for the site (singleton)
// @Tags         site-setting
// @Accept       json
// @Produce      json
// @Param        body  body      dto.UpdateSiteScriptsRequest  true  "Scripts"
// @Success      200   {object}  entity.SiteSetting
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/site-setting/scripts [put]
func (h *SiteSettingHandler) UpdateScripts(c *gin.Context) {
	before, _ := h.svc.Get()

	var req dto.UpdateSiteScriptsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data script site tidak valid"})
		return
	}

	item, err := h.svc.UpdateScripts(&req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidSiteScript) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Script site tidak valid"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan script site"})
		return
	}
	if before != nil && item != nil {
		removeSiteAssetIfChanged(before.LogoURL, item.LogoURL)
		removeSiteAssetIfChanged(before.FaviconURL, item.FaviconURL)
		removeSiteAssetIfChanged(before.DefaultImage, item.DefaultImage)
	}
	c.JSON(http.StatusOK, item)
}

// PostDateUrlRecycleRunNow appends the submitted URLs to the in-memory queue.
// The background worker (daterecycle.TickScheduled) drains it one URL per interval.
// URLs are NOT persisted; they live only until processed or the app restarts.
func (h *SiteSettingHandler) PostDateUrlRecycleRunNow(c *gin.Context) {
	ss, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ss.PostDateUrlRecycleEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Aktifkan recycle URL terlebih dulu."})
		return
	}
	var req dto.PostDateUrlRecycleRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}
	urls := daterecycle.SplitURLs(req.URLs)
	if len(urls) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tambahkan minimal satu URL (satu per baris) terlebih dulu."})
		return
	}
	added, total := daterecycle.EnqueueURLs(urls)
	c.JSON(http.StatusAccepted, gin.H{
		"message":     "Recycle URL masuk antrean",
		"queued":      added,
		"queue_total": total,
	})
}

func removeSiteAssetIfChanged(oldVal, newVal string) {
	oldURL := strings.TrimSpace(oldVal)
	newURL := strings.TrimSpace(newVal)
	if oldURL == "" || oldURL == newURL {
		return
	}
	removeUploadedFileIfLocal(oldURL, siteAssetURLBase, siteAssetDir)
}
