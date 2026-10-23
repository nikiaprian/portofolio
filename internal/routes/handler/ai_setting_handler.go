package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appconfig "github.com/nikiaprian/tirtayasa/config"
	"github.com/nikiaprian/tirtayasa/internal/routes/dto"
	"github.com/nikiaprian/tirtayasa/internal/services"
)

// AISettingHandler serves the singleton AI settings resource.
type AISettingHandler struct {
	svc services.AISettingService
	db  *gorm.DB
	cfg appconfig.Config
}

// NewAISettingHandler constructs AISettingHandler.
func NewAISettingHandler(svc services.AISettingService, db *gorm.DB, cfg appconfig.Config) *AISettingHandler {
	return &AISettingHandler{svc: svc, db: db, cfg: cfg}
}

// Get
// @Summary      Get AI settings row
// @Description  Returns gemini, groq, openai keys and optional model names from ai_setting
// @Tags         ai-setting
// @Produce      json
// @Success      200  {object}  entity.AISetting
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/ai-setting [get]
func (h *AISettingHandler) Get(c *gin.Context) {
	item, err := h.svc.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Update
// @Summary      Update AI settings
// @Description  Updates ai_setting: keys and models are taken from the body; prompt fields may be omitted to keep existing stored text.
// @Tags         ai-setting
// @Accept       json
// @Produce      json
// @Param        body  body      dto.UpdateAISettingRequest  true  "Keys and models"
// @Success      200   {object}  entity.AISetting
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/ai-setting [put]
func (h *AISettingHandler) Update(c *gin.Context) {
	before, _ := h.svc.Get()

	var req dto.UpdateAISettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data pengaturan AI tidak valid"})
		return
	}

	item, err := h.svc.Update(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan pengaturan AI"})
		return
	}
	if before != nil && item != nil {
		removeBulkCoverTemplatesIfRemoved(before.BulkCoverBackgroundURL, item.BulkCoverBackgroundURL)
	}
	c.JSON(http.StatusOK, item)
}

// UpdateBulkPostPromptTemplate updates all bulk-post prompt fields (bulk posts admin UI).
func (h *AISettingHandler) UpdateBulkPostPromptTemplate(c *gin.Context) {
	before, _ := h.svc.Get()

	var req dto.UpdateBulkPostPromptsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data prompt bulk post tidak valid"})
		return
	}
	item, err := h.svc.UpdateBulkPostPrompts(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan prompt bulk post"})
		return
	}
	if before != nil && item != nil {
		removeBulkCoverTemplatesIfRemoved(before.BulkCoverBackgroundURL, item.BulkCoverBackgroundURL)
	}
	c.JSON(http.StatusOK, gin.H{
		"generate_prompt_title":            item.GeneratePromptTitle,
		"generate_prompt_content_name":     item.GeneratePromptContentName,
		"generate_prompt_content":          item.GeneratePromptContent,
		"generate_prompt_content_name_2":   item.GeneratePromptContentName2,
		"generate_prompt_content_2":        item.GeneratePromptContent2,
		"generate_prompt_content_name_3":   item.GeneratePromptContentName3,
		"generate_prompt_content_3":        item.GeneratePromptContent3,
		"generate_prompt_meta_description": item.GeneratePromptMetaDescription,
		"generate_prompt_meta_keywords":    item.GeneratePromptMetaKeywords,
		"generate_prompt_image":            item.GeneratePromptImage,
		"generate_prompt_image_to_content": item.GeneratePromptImageToContent,
		"bulk_cover_background_url":        item.BulkCoverBackgroundURL,
	})
}

func removeBulkCoverTemplatesIfRemoved(oldRaw, newRaw string) {
	oldURLs := parseBulkTemplateURLs(oldRaw)
	if len(oldURLs) == 0 {
		return
	}
	newSet := make(map[string]struct{}, 16)
	for _, u := range parseBulkTemplateURLs(newRaw) {
		v := strings.TrimSpace(u)
		if v == "" {
			continue
		}
		newSet[v] = struct{}{}
	}
	for _, oldURL := range oldURLs {
		v := strings.TrimSpace(oldURL)
		if v == "" {
			continue
		}
		if _, exists := newSet[v]; exists {
			continue
		}
		removeUploadedFileIfLocal(v, bulkCoverTplURLBase, bulkCoverTplDir)
	}
}
