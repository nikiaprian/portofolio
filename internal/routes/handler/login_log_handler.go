package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// LoginLogHandler lists sign-in audit rows (admin only).
type LoginLogHandler struct {
	db *gorm.DB
}

// NewLoginLogHandler constructs LoginLogHandler.
func NewLoginLogHandler(db *gorm.DB) *LoginLogHandler {
	return &LoginLogHandler{db: db}
}

// List returns recent login_logs (newest first).
// @Summary      List login logs
// @Description  Paginated login_logs for admin audit. Rows are pruned on the same rolling window as scraper and bulk post logs (ADMIN_LOG_RETENTION_HOURS).
// @Tags         auth
// @Produce      json
// @Param        page     query     int     false  "Page (1-based)"
// @Param        size     query     int     false  "Page size (max 500)"
// @Param        success  query     string  false  "Filter: 1/true or 0/false"
// @Param        q        query     string  false  "Search email, IP, user agent, or failure reason"
// @Success      200      {object}  map[string]interface{}
// @Failure      500      {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/login-logs [get]
func (h *LoginLogHandler) List(c *gin.Context) {
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

	succQ := strings.ToLower(strings.TrimSpace(c.Query("success")))
	base := h.db.Model(&entity.LoginLog{})
	switch succQ {
	case "1", "true", "yes", "ok":
		base = base.Where("success = ?", true)
	case "0", "false", "no":
		base = base.Where("success = ?", false)
	}
	if pat, ok := logSearchLike(c.Query("q")); ok {
		base = base.Where(
			"(email LIKE ? OR failure_reason LIKE ? OR ip LIKE ? OR user_agent LIKE ? OR COALESCE(CAST(user_id AS CHAR), '') LIKE ?)",
			pat, pat, pat, pat, pat,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log login"})
		return
	}
	var items []entity.LoginLog
	q2 := h.db.Model(&entity.LoginLog{})
	switch succQ {
	case "1", "true", "yes", "ok":
		q2 = q2.Where("success = ?", true)
	case "0", "false", "no":
		q2 = q2.Where("success = ?", false)
	}
	if pat, ok := logSearchLike(c.Query("q")); ok {
		q2 = q2.Where(
			"(email LIKE ? OR failure_reason LIKE ? OR ip LIKE ? OR user_agent LIKE ? OR COALESCE(CAST(user_id AS CHAR), '') LIKE ?)",
			pat, pat, pat, pat, pat,
		)
	}
	if err := q2.Order("id DESC").Limit(size).Offset(offset).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log login"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// Clear deletes all login log rows. Admin-only route.
func (h *LoginLogHandler) Clear(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database tidak tersedia"})
		return
	}
	res := h.db.Where("1 = 1").Delete(&entity.LoginLog{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membersihkan log login"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": res.RowsAffected})
}
