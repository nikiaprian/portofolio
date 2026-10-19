package entity

import (
	"strings"
	"time"
)

// OpenCodeLocalSetting is stored as one JSON column on ai_setting.
type OpenCodeLocalSetting struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Model    string `json:"model"`
}

// Configured reports whether local OpenCode connection fields are set.
func (c OpenCodeLocalSetting) Configured() bool {
	return strings.TrimSpace(c.Host) != "" || strings.TrimSpace(c.Port) != ""
}

// Normalize trims fields and defaults model to auto when empty.
func (c *OpenCodeLocalSetting) Normalize() {
	if c == nil {
		return
	}
	c.Host = strings.TrimSpace(c.Host)
	c.Port = strings.TrimSpace(c.Port)
	c.Username = strings.TrimSpace(c.Username)
	c.Password = strings.TrimSpace(c.Password)
	c.Model = strings.TrimSpace(c.Model)
	if c.Model == "" {
		c.Model = "auto"
	}
}

// AISetting stores AI provider API keys and optional model names (singleton row in ai_setting).
type AISetting struct {
	ID       uint   `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Gemini   string `gorm:"type:text;column:gemini" json:"gemini"`
	Groq     string `gorm:"type:text;column:groq" json:"groq"`
	OpenAI   string `gorm:"type:text;column:openai" json:"openai"`
	Claude   string `gorm:"type:text;column:claude" json:"claude"`
	OpenCode string `gorm:"type:text;column:opencode" json:"opencode"`
	// OpenCodeLocal holds host/port/auth/model in one JSON column.
	OpenCodeLocal                 OpenCodeLocalSetting `gorm:"column:opencode_local;type:json;serializer:json" json:"opencode_local"`
	GeminiModel                   string               `gorm:"size:255;column:gemini_model" json:"gemini_model"`
	GroqModel                     string               `gorm:"size:255;column:groq_model" json:"groq_model"`
	OpenAIModel                   string               `gorm:"size:255;column:openai_model" json:"openai_model"`
	ClaudeModel                   string               `gorm:"size:255;column:claude_model" json:"claude_model"`
	OpenCodeModel                 string               `gorm:"size:255;column:opencode_model" json:"opencode_model"`
	GeminiSearchMode              bool                 `gorm:"column:gemini_search_mode;default:false" json:"gemini_search_mode"`
	OpenAISearchMode              bool                 `gorm:"column:openai_search_mode;default:false" json:"openai_search_mode"`
	GroqSearchMode                bool                 `gorm:"column:groq_search_mode;default:false" json:"groq_search_mode"`
	ClaudeSearchMode              bool                 `gorm:"column:claude_search_mode;default:false" json:"claude_search_mode"`
	OpenCodeSearchMode            bool                 `gorm:"column:opencode_search_mode;default:false" json:"opencode_search_mode"`
	ScraperAIProvider             string               `gorm:"column:scraper_ai_provider;size:20;default:auto" json:"scraper_ai_provider"`
	ScraperPromptTitle            string               `gorm:"column:scraper_prompt_title;type:text" json:"scraper_prompt_title"`
	ScraperPromptContent          string               `gorm:"column:scraper_prompt_content;type:text" json:"scraper_prompt_content"`
	GenerateAIProvider            string               `gorm:"column:generate_ai_provider;size:20;default:auto" json:"generate_ai_provider"`
	GeneratePromptTitle           string               `gorm:"column:generate_prompt_title;type:text" json:"generate_prompt_title"`
	GeneratePromptContentName     string               `gorm:"column:generate_prompt_content_name;size:120" json:"generate_prompt_content_name"`
	GeneratePromptContent         string               `gorm:"column:generate_prompt_content;type:text" json:"generate_prompt_content"`
	GeneratePromptContentName2    string               `gorm:"column:generate_prompt_content_name_2;size:120" json:"generate_prompt_content_name_2"`
	GeneratePromptContent2        string               `gorm:"column:generate_prompt_content_2;type:text" json:"generate_prompt_content_2"`
	GeneratePromptContentName3    string               `gorm:"column:generate_prompt_content_name_3;size:120" json:"generate_prompt_content_name_3"`
	GeneratePromptContent3        string               `gorm:"column:generate_prompt_content_3;type:text" json:"generate_prompt_content_3"`
	GeneratePromptMetaDescription string               `gorm:"column:generate_prompt_meta_description;type:text" json:"generate_prompt_meta_description"`
	GeneratePromptMetaKeywords    string               `gorm:"column:generate_prompt_meta_keywords;type:text" json:"generate_prompt_meta_keywords"`
	GeneratePromptImage           string               `gorm:"column:generate_prompt_image;type:text" json:"generate_prompt_image"`
	// GeneratePromptImageToContent: default template for AI images inserted into article body (manual/bulk/scraper).
	GeneratePromptImageToContent string `gorm:"column:generate_prompt_image_to_content;type:text" json:"generate_prompt_image_to_content"`
	// BulkCoverBackgroundURL: optional template URL list (newline-separated) for server-rendered bulk post covers.
	BulkCoverBackgroundURL string    `gorm:"column:bulk_cover_background_url;type:text" json:"bulk_cover_background_url"`
	CreatedAt              time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt              time.Time `json:"updated_at" swaggerignore:"true"`
}

// TableName keeps the GORM table name as ai_setting (singular).
func (AISetting) TableName() string {
	return "ai_setting"
}
