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

// ManualPostAILogHandler lists manual post AI generate audit rows.
// Writers see only their own rows.
type ManualPostAILogHandler struct {
	repo repositories.ManualPostAILogRepository
}

// NewManualPostAILogHandler constructs ManualPostAILogHandler.
func NewManualPostAILogHandler(db *gorm.DB) *ManualPostAILogHandler {
	return &ManualPostAILogHandler{repo: repositories.NewManualPostAILogRepository(db)}
}

type manualPostAILogAPIItem struct {
	ID            uint      `json:"id"`
	UserID        uint      `json:"user_id"`
	UserDisplay   string    `json:"user_display"`
	Level         string    `json:"level"`
	Event         string    `json:"event"`
	Message       string    `json:"message"`
	PostID        *uint     `json:"post_id,omitempty"`
	Title         string    `json:"title"`
	ProviderUsed  string    `json:"provider_used"`
	ProviderInput string    `json:"provider_input"`
	CreatedAt     time.Time `json:"created_at"`
}

// List returns recent manual_post_ai_logs with user display.
func (h *ManualPostAILogHandler) List(c *gin.Context) {
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

	rows, total, err := h.repo.FindPaginated(repositories.ManualPostAILogListFilter{
		RequesterID: requesterID,
		WriterOnly:  strings.EqualFold(strings.TrimSpace(requesterRole), "writer"),
		Level:       c.Query("level"),
		EventPrefix: c.Query("event"),
		Search:      c.Query("q"),
		Page:        page,
		Size:        size,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat log AI post manual"})
		return
	}

	items := make([]manualPostAILogAPIItem, 0, len(rows))
	for _, row := range rows {
		pu, msg := AugmentInlineLogProviders(row.Message, row.ProviderUsed)
		items = append(items, manualPostAILogAPIItem{
			ID:            row.ID,
			UserID:        row.UserID,
			UserDisplay:   bulkPostUserDisplay(row.User.Name, row.User.Email),
			Level:         row.Level,
			Event:         row.Event,
			Message:       msg,
			PostID:        row.PostID,
			Title:         strings.TrimSpace(row.Title),
			ProviderUsed:  strings.TrimSpace(pu),
			ProviderInput: strings.TrimSpace(row.ProviderInput),
			CreatedAt:     row.CreatedAt,
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

// Clear deletes all manual post AI log rows. Admin-only route.
func (h *ManualPostAILogHandler) Clear(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database tidak tersedia"})
		return
	}
	deleted, err := h.repo.ClearAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membersihkan log AI post manual"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}
