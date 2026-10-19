package dto

import "github.com/Bebii-Digital/bebii-blog/internal/entity"

// UpdateAISettingRequest is the body for PUT /admin/ai-setting.
// API keys and model fields are always applied from the JSON body.
// Prompt fields use *string: omit a key to leave the stored value unchanged; send "" to clear.
type UpdateAISettingRequest struct {
	Gemini      string `json:"gemini"`
	Groq        string `json:"groq"`
	OpenAI      string `json:"openai"`
	Claude      string `json:"claude"`
	OpenCode    string `json:"opencode"`
	OpenCodeLocal entity.OpenCodeLocalSetting `json:"opencode_local"`
	GeminiModel string `json:"gemini_model"`
	GroqModel   string `json:"groq_model"`
	OpenAIModel string `json:"openai_model"`
	ClaudeModel string `json:"claude_model"`
	OpenCodeModel string `json:"opencode_model"`
	GeminiSearchMode   *bool `json:"gemini_search_mode"`
	OpenAISearchMode   *bool `json:"openai_search_mode"`
	GroqSearchMode     *bool `json:"groq_search_mode"`
	ClaudeSearchMode   *bool `json:"claude_search_mode"`
	OpenCodeSearchMode *bool `json:"opencode_search_mode"`
	// Scraper: news feed paraphrasing
	ScraperAIProvider    string  `json:"scraper_ai_provider" binding:"omitempty,oneof=auto gemini openai groq claude opencode opencode_local"`
	ScraperPromptTitle   *string `json:"scraper_prompt_title" binding:"omitempty,max=20000"`
	ScraperPromptContent *string `json:"scraper_prompt_content" binding:"omitempty,max=20000"`
	// Generate: bulk post / title suggest templates
	GenerateAIProvider            string  `json:"generate_ai_provider" binding:"omitempty,oneof=auto gemini openai groq claude opencode opencode_local"`
	GeneratePromptTitle           *string `json:"generate_prompt_title" binding:"omitempty,max=20000"`
	GeneratePromptContentName     *string `json:"generate_prompt_content_name" binding:"omitempty,max=120"`
	GeneratePromptContent         *string `json:"generate_prompt_content" binding:"omitempty,max=20000"`
	GeneratePromptContentName2    *string `json:"generate_prompt_content_name_2" binding:"omitempty,max=120"`
	GeneratePromptContent2        *string `json:"generate_prompt_content_2" binding:"omitempty,max=20000"`
	GeneratePromptContentName3    *string `json:"generate_prompt_content_name_3" binding:"omitempty,max=120"`
	GeneratePromptContent3        *string `json:"generate_prompt_content_3" binding:"omitempty,max=20000"`
	GeneratePromptMetaDescription *string `json:"generate_prompt_meta_description" binding:"omitempty,max=20000"`
	GeneratePromptMetaKeywords    *string `json:"generate_prompt_meta_keywords" binding:"omitempty,max=20000"`
	GeneratePromptImage           *string `json:"generate_prompt_image" binding:"omitempty,max=20000"`
	GeneratePromptImageToContent  *string `json:"generate_prompt_image_to_content" binding:"omitempty,max=20000"`
	// BulkCoverBackgroundURL: optional template URL list (newline-separated); empty = accent gradient only.
	BulkCoverBackgroundURL string `json:"bulk_cover_background_url" binding:"max=20000"`
}

// TestChatRequest is the body for POST /admin/ai-setting/test-chat.
type TestChatRequest struct {
	Provider string `json:"provider" binding:"required,oneof=gemini openai groq claude opencode opencode_local"`
	Prompt   string `json:"prompt"`
}

// TestChatResponse is the response for a successful test chat.
type TestChatResponse struct {
	Response string `json:"response"`
}

// ListOpenCodeLocalModelsRequest is the body for POST /admin/ai-setting/opencode-local/models.
// Empty fields fall back to saved ai_setting values.
type ListOpenCodeLocalModelsRequest struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// UpdateBulkPostPromptsRequest is the body for PUT /admin/ai-setting/bulk-post-template.
// Pointer fields: only non-null JSON keys are applied (merge); omitted keys keep DB values.
type UpdateBulkPostPromptsRequest struct {
	GeneratePromptTitle           *string `json:"generate_prompt_title" binding:"omitempty,max=20000"`
	GeneratePromptContentName     *string `json:"generate_prompt_content_name" binding:"omitempty,max=120"`
	GeneratePromptContent         *string `json:"generate_prompt_content" binding:"omitempty,max=20000"`
	GeneratePromptContentName2    *string `json:"generate_prompt_content_name_2" binding:"omitempty,max=120"`
	GeneratePromptContent2        *string `json:"generate_prompt_content_2" binding:"omitempty,max=20000"`
	GeneratePromptContentName3    *string `json:"generate_prompt_content_name_3" binding:"omitempty,max=120"`
	GeneratePromptContent3        *string `json:"generate_prompt_content_3" binding:"omitempty,max=20000"`
	GeneratePromptMetaDescription *string `json:"generate_prompt_meta_description" binding:"omitempty,max=20000"`
	GeneratePromptMetaKeywords    *string `json:"generate_prompt_meta_keywords" binding:"omitempty,max=20000"`
	GeneratePromptImage           *string `json:"generate_prompt_image" binding:"omitempty,max=20000"`
	GeneratePromptImageToContent  *string `json:"generate_prompt_image_to_content" binding:"omitempty,max=20000"`
	BulkCoverBackgroundURL        *string `json:"bulk_cover_background_url" binding:"omitempty,max=20000"`
}
