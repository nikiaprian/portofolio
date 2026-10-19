package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/ai"
	"github.com/Bebii-Digital/bebii-blog/internal/ai/opencode"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
	"github.com/Bebii-Digital/bebii-blog/internal/services"
)

// AISettingHandler serves the singleton AI settings resource.
type AISettingHandler struct {
	svc services.AISettingService
	db  *gorm.DB
	cfg appconfig.Config
	gen *ai.Generator
}

// NewAISettingHandler constructs AISettingHandler.
func NewAISettingHandler(svc services.AISettingService, db *gorm.DB, cfg appconfig.Config, gen *ai.Generator) *AISettingHandler {
	return &AISettingHandler{svc: svc, db: db, cfg: cfg, gen: gen}
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

// TestChat sends a simple prompt to the specified AI provider and returns the response.
func (h *AISettingHandler) TestChat(c *gin.Context) {
	var req dto.TestChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider tidak valid. Pilihan: gemini, openai, groq, claude, opencode, opencode_local"})
		return
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Hello! Reply with just 'OK' if you receive this message."
	}

	timeout := 30 * time.Second
	if req.Provider == "opencode_local" {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	var provider ai.Provider
	switch req.Provider {
	case "gemini":
		provider = ai.ProviderGemini
	case "openai":
		provider = ai.ProviderOpenAI
	case "groq":
		provider = ai.ProviderGroq
	case "claude":
		provider = ai.ProviderClaude
	case "opencode":
		provider = ai.ProviderOpenCode
	case "opencode_local":
		provider = ai.ProviderOpenCodeLocal
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider tidak dikenal"})
		return
	}

	text, err := h.gen.GenerateWith(ctx, provider, prompt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto.TestChatResponse{Response: text})
}

// ListOpenCodeLocalModels proxies model listing from a local OpenCode server.
func (h *AISettingHandler) ListOpenCodeLocalModels(c *gin.Context) {
	var req dto.ListOpenCodeLocalModelsRequest
	_ = c.ShouldBindJSON(&req)

	host := strings.TrimSpace(req.Host)
	port := strings.TrimSpace(req.Port)
	user := strings.TrimSpace(req.Username)
	pass := strings.TrimSpace(req.Password)
	if host == "" && port == "" {
		if item, err := h.svc.Get(); err == nil && item != nil {
			host = strings.TrimSpace(item.OpenCodeLocal.Host)
			port = strings.TrimSpace(item.OpenCodeLocal.Port)
			if user == "" {
				user = strings.TrimSpace(item.OpenCodeLocal.Username)
			}
			if pass == "" {
				pass = strings.TrimSpace(item.OpenCodeLocal.Password)
			}
		}
	}
	if host == "" && port == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Isi host/port OpenCode local terlebih dahulu"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	out, err := opencode.ListLocalModels(ctx, opencode.LocalConfig{
		Host:     host,
		Port:     port,
		Username: user,
		Password: pass,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
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
