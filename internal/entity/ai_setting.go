package entity

import "time"

// AISetting stores AI provider API keys and optional model names (singleton row in ai_setting).
type AISetting struct {
	ID                            uint   `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Gemini                        string `gorm:"type:text;column:gemini" json:"gemini"`
	Groq                          string `gorm:"type:text;column:groq" json:"groq"`
	OpenAI                        string `gorm:"type:text;column:openai" json:"openai"`
	Claude                        string `gorm:"type:text;column:claude" json:"claude"`
	GeminiModel                   string `gorm:"size:255;column:gemini_model" json:"gemini_model"`
	GroqModel                     string `gorm:"size:255;column:groq_model" json:"groq_model"`
	OpenAIModel                   string `gorm:"size:255;column:openai_model" json:"openai_model"`
	ClaudeModel                   string `gorm:"size:255;column:claude_model" json:"claude_model"`
	ScraperAIProvider             string `gorm:"column:scraper_ai_provider;size:20;default:auto" json:"scraper_ai_provider"`
	ScraperPromptTitle            string `gorm:"column:scraper_prompt_title;type:text" json:"scraper_prompt_title"`
	ScraperPromptContent          string `gorm:"column:scraper_prompt_content;type:text" json:"scraper_prompt_content"`
	GenerateAIProvider            string `gorm:"column:generate_ai_provider;size:20;default:auto" json:"generate_ai_provider"`
	GeneratePromptTitle           string `gorm:"column:generate_prompt_title;type:text" json:"generate_prompt_title"`
	GeneratePromptContentName     string `gorm:"column:generate_prompt_content_name;size:120" json:"generate_prompt_content_name"`
	GeneratePromptContent         string `gorm:"column:generate_prompt_content;type:text" json:"generate_prompt_content"`
	GeneratePromptContentName2    string `gorm:"column:generate_prompt_content_name_2;size:120" json:"generate_prompt_content_name_2"`
	GeneratePromptContent2        string `gorm:"column:generate_prompt_content_2;type:text" json:"generate_prompt_content_2"`
	GeneratePromptContentName3    string `gorm:"column:generate_prompt_content_name_3;size:120" json:"generate_prompt_content_name_3"`
	GeneratePromptContent3        string `gorm:"column:generate_prompt_content_3;type:text" json:"generate_prompt_content_3"`
	GeneratePromptMetaDescription string `gorm:"column:generate_prompt_meta_description;type:text" json:"generate_prompt_meta_description"`
	GeneratePromptMetaKeywords    string `gorm:"column:generate_prompt_meta_keywords;type:text" json:"generate_prompt_meta_keywords"`
	GeneratePromptImage           string `gorm:"column:generate_prompt_image;type:text" json:"generate_prompt_image"`
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
