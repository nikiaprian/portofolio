package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/repositories"
)

// BulkPostLogHandler lists bulk AI post audit rows (newest first). Writers see only their rows.
type BulkPostLogHandler struct {
	repo repositories.BulkPostLogRepository
}

// NewBulkPostLogHandler constructs BulkPostLogHandler.
func NewBulkPostLogHandler(db *gorm.DB) *BulkPostLogHandler {
	return &BulkPostLogHandler{repo: repositories.NewBulkPostLogRepository(db)}
}

type bulkPostLogAPIItem struct {
	ID           uint      `json:"id"`
	UserID       uint      `json:"user_id"`
	UserDisplay  string    `json:"user_display"`
	Level        string    `json:"level"`
	Event        string    `json:"event"`
	Message      string    `json:"message"`
	CategoryID   *uint     `json:"category_id,omitempty"`
	CategoryName string    `json:"category_name"`
	PostID       *uint     `json:"post_id,omitempty"`
	Title        string    `json:"title"`
	ProviderUsed string    `json:"provider_used"`
	CreatedAt    time.Time `json:"created_at"`
}

func bulkPostUserDisplay(name, email string) string {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name != "" && email != "" {
		return name + " (" + email + ")"
	}
	if email != "" {
		return email
	}
	if name != "" {
		return name
	}
	return "—"
}

// List returns recent bulk_post_logs with user and category names.
// @Summary      List bulk post logs
// @Description  Paginated bulk_post_logs; writers only see their own entries. Rows are pruned on the same rolling window as scraper and login logs (ADMIN_LOG_RETENTION_HOURS).
// @Tags         posts
// @Produce      json
// @Param        page   query     int     false  "Page (1-based)"
// @Param        size   query     int     false  "Page size (max 500)"
// @Param        level  query     string  false  "Filter by level (info, warn, error)"
// @Param        event  query     string  false  "Filter by event prefix"
// @Param        q      query     string  false  "Search message, title, event, user, or category"
// @Success      200    {object}  map[string]interface{}
// @Failure      500    {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/bulk-post-logs [get]
func (h *BulkPostLogHandler) List(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database tidak tersedia"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))

	requesterID, requesterRole := requesterFromContext(c)
	if requesterID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi tidak valid"})
		return
	}

	rows, total, err := h.repo.FindPaginated(repositories.BulkPostLogListFilter{
		RequesterID: requesterID,
		WriterOnly:  strings.EqualFold(strings.TrimSpace(requesterRole), "writer"),
		Level:       c.Query("level"),
		EventPrefix: c.Query("event"),
		Search:      c.Query("q"),
		Page:        page,
		Size:        size,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log bulk post"})
		return
	}

	items := make([]bulkPostLogAPIItem, 0, len(rows))
	for _, row := range rows {
		catName := ""
		if row.Category != nil {
			catName = strings.TrimSpace(row.Category.Name)
		}
		pu, msg := AugmentInlineLogProviders(row.Message, row.ProviderUsed)
		items = append(items, bulkPostLogAPIItem{
			ID:           row.ID,
			UserID:       row.UserID,
			UserDisplay:  bulkPostUserDisplay(row.User.Name, row.User.Email),
			Level:        row.Level,
			Event:        row.Event,
			Message:      msg,
			CategoryID:   row.CategoryID,
			CategoryName: catName,
			PostID:       row.PostID,
			Title:        row.Title,
			ProviderUsed: pu,
			CreatedAt:    row.CreatedAt,
		})
	}

	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 500 {
		size = 500
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// Clear deletes all bulk post log rows. Admin-only route.
func (h *BulkPostLogHandler) Clear(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database tidak tersedia"})
		return
	}
	deleted, err := h.repo.ClearAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membersihkan log bulk post"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}
