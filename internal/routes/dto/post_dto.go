package dto

import "time"

// CreatePostRequest validation
type CreatePostRequest struct {
	Title             string     `json:"title" binding:"required,min=3,max=200"`
	Slug              string     `json:"slug" binding:"required,min=3,max=255,slug"`
	Content           string     `json:"content" binding:"required,min=10,max=100000"`
	Status            string     `json:"status" binding:"required,oneof=draft published scheduled"`
	PublishAt         *time.Time `json:"publish_at,omitempty"`
	Image             *string    `json:"image" binding:"omitempty,max=500"`
	ImageOrigin       string     `json:"image_origin" binding:"omitempty,oneof=scrape web_search template ai manual"`
	ImageOriginDetail string     `json:"image_origin_detail" binding:"omitempty,max=200"`
	Source            *string    `json:"source" binding:"omitempty,max=4096"`
	IncludeSource     *bool      `json:"include_source,omitempty"`
	AuthorID          uint       `json:"author_id" binding:"required,gt=0"`
	CategoryID        uint       `json:"category_id" binding:"required,gt=0"`
	Tags              []uint     `json:"tags" binding:"omitempty,dive,gt=0"`
	// Optional SEO overrides; empty or omitted uses auto-filled meta from title/body/tags.
	MetaDescription *string `json:"meta_description" binding:"omitempty,max=500"`
	MetaKeywords    *string `json:"meta_keywords" binding:"omitempty,max=500"`
}

// UpdatePostRequest validation
type UpdatePostRequest struct {
	Title             *string    `json:"title" binding:"required,min=3,max=200"`
	Slug              *string    `json:"slug" binding:"required,min=3,max=255,slug"`
	Content           *string    `json:"content" binding:"required,min=10,max=100000"`
	Status            *string    `json:"status" binding:"required,oneof=draft published scheduled"`
	PublishAt         *time.Time `json:"publish_at,omitempty"`
	Image             *string    `json:"image" binding:"omitempty,max=500"`
	ImageOrigin       string     `json:"image_origin" binding:"omitempty,oneof=scrape web_search template ai manual"`
	ImageOriginDetail string     `json:"image_origin_detail" binding:"omitempty,max=200"`
	Source            *string    `json:"source" binding:"omitempty,max=4096"`
	IncludeSource     *bool      `json:"include_source,omitempty"`
	AuthorID          *uint      `json:"author_id" binding:"required,gt=0"`
	CategoryID        *uint      `json:"category_id" binding:"required,gt=0"`
	Tags              []uint     `json:"tags" binding:"omitempty,dive,gt=0"`
	MetaDescription   *string    `json:"meta_description" binding:"omitempty,max=500"`
	MetaKeywords      *string    `json:"meta_keywords" binding:"omitempty,max=500"`
}

// GeneratePostDraftRequest is the body for AI-assisted draft generation.
type GeneratePostDraftRequest struct {
	// Prompt is optional; when empty the backend derives one from AI settings + title.
	Prompt   string `json:"prompt" binding:"omitempty,max=12000"`
	Title    string `json:"title" binding:"omitempty,min=3,max=220"`
	Provider string `json:"provider" binding:"omitempty,oneof=auto openai gemini groq claude"`
	// PromptContentVariant chooses AI setting template: 1=generate_prompt_content (default), 2=generate_prompt_content_2, 3=generate_prompt_content_3.
	PromptContentVariant int `json:"prompt_content_variant" binding:"omitempty,oneof=1 2 3"`
	// Tags optional; tag names are passed into meta keyword/description prompt placeholders ({{keywords}}).
	Tags []uint `json:"tags" binding:"omitempty,dive,gt=0"`
}

// GeneratePostImageRequest is the body for AI-assisted image generation (cover or in-content).
type GeneratePostImageRequest struct {
	Title    string `json:"title" binding:"required,min=3,max=220"`
	Prompt   string `json:"prompt" binding:"omitempty,max=4000"`
	Provider string `json:"provider" binding:"omitempty,oneof=auto openai gemini groq claude"`
	// Purpose: "cover" (default) uses generate_prompt_image; "content" uses generate_prompt_image_to_content when prompt is empty.
	Purpose string `json:"purpose" binding:"omitempty,oneof=cover content"`
	// Source: "ai" (default) generates an AI image; "web_search" downloads a relevant public web image by title.
	Source string `json:"source" binding:"omitempty,oneof=ai web_search"`
	// SearchAttempt is used by manual web search so repeated clicks can skip earlier candidates.
	SearchAttempt int `json:"search_attempt" binding:"omitempty,min=0,max=50"`
	// WebSearchSource limits manual web_search to one source; empty/auto searches all sources.
	WebSearchSource string `json:"web_search_source" binding:"omitempty,oneof=auto openverse wikimedia unsplash pixabay pexels wordpress"`
	// PreviousImage is deleted after a successful manual web_search replacement when it belongs to post storage.
	PreviousImage string `json:"previous_image" binding:"omitempty,max=500"`
}

// BulkSuggestTitlesRequest asks the AI for article title lines from a keyword.
type BulkSuggestTitlesRequest struct {
	// Keyword may be a long seed phrase (not only a short SEO term).
	Keyword  string `json:"keyword" binding:"required,min=2,max=8192"`
	Count    int    `json:"count" binding:"required,min=1,max=50"`
	Provider string `json:"provider" binding:"omitempty,oneof=auto openai gemini groq claude"`
}

// BulkRegenerateTitlesRequest asks the AI to rewrite each line of titles_text (same line count).
// TitlesText max is rune-based (validator); must fit up to bulkMaxArticleTitleLines with long lines per handler.
type BulkRegenerateTitlesRequest struct {
	TitlesText string `json:"titles_text" binding:"required,max=2097152"`
	Keyword    string `json:"keyword" binding:"omitempty,max=8192"`
	Provider   string `json:"provider" binding:"omitempty,oneof=auto openai gemini groq claude"`
}

// BulkScrapeXMLTitlesRequest parses RSS/Atom/WXR XML for titles. Body may include a public feed URL and/or raw xml (API); admin UI uses URL only.
type BulkScrapeXMLTitlesRequest struct {
	URL string `json:"url" binding:"omitempty,max=2048"`
	XML string `json:"xml" binding:"omitempty,max=4000000"`
}

// BulkCheckTitlesRequest checks bulk title lines for near-duplicates (default ≥90% similarity).
type BulkCheckTitlesRequest struct {
	TitlesText string `json:"titles_text" binding:"required,max=2097152"`
}

// BulkTitleSimilarityMatch pairs a bulk line (new) with an existing title (old).
type BulkTitleSimilarityMatch struct {
	Index      int     `json:"index"`
	NewTitle   string  `json:"new_title"`
	OldTitle   string  `json:"old_title"`
	Source     string  `json:"source"` // database | list
	Similarity float64 `json:"similarity"`
	PostID     *uint   `json:"post_id,omitempty"`
}

// BulkCheckTitlesResponse lists similar title pairs found in the bulk list.
type BulkCheckTitlesResponse struct {
	Threshold float64                    `json:"threshold"`
	Count     int                        `json:"count"`
	Matches   []BulkTitleSimilarityMatch `json:"matches"`
}

// BulkGeneratePostsRequest creates posts from suggested titles using the bulk prompt template in AI settings.
type BulkGeneratePostsRequest struct {
	Keyword string   `json:"keyword" binding:"required,min=1,max=8192"`
	Titles  []string `json:"titles" binding:"required"`
	// CategoryID is the fallback category for the whole batch. CategoryIDs or CategoryRefs can override it per title.
	CategoryID uint   `json:"category_id" binding:"omitempty,gt=0"`
	AuthorID   uint   `json:"author_id" binding:"required,gt=0"`
	Tags       []uint `json:"tags" binding:"omitempty,dive,gt=0"`
	// Slugs optional; when must match len(Titles). Empty string at index = derive slug from title.
	Slugs []string `json:"slugs"`
	// CategoryIDs optional; when set must match len(Titles). Each entry is used for the title at the same index.
	CategoryIDs []uint `json:"category_ids" binding:"omitempty,dive,gt=0"`
	// CategoryRefs optional; when set must match len(Titles). Each entry may be a category id, slug, or name (case-insensitive).
	CategoryRefs []string `json:"category_refs"`
	// IntervalMinutes, BatchBaseTime, TitleIndex: used only when status is scheduled (staggered publish_at).
	IntervalMinutes int `json:"interval_minutes" binding:"min=0,max=10080"`
	// GenerateDelayMinutes waits between each AI generation in a bulk run (rate limit / prompt refresh). 0 means default 1 minute.
	GenerateDelayMinutes int    `json:"generate_delay_minutes" binding:"min=0,max=120"`
	Provider             string `json:"provider" binding:"omitempty,oneof=auto openai gemini groq claude"`
	// PromptContentVariant chooses AI setting template: 1=generate_prompt_content (default), 2=generate_prompt_content_2, 3=generate_prompt_content_3.
	PromptContentVariant int    `json:"prompt_content_variant" binding:"omitempty,oneof=1 2 3"`
	Status               string `json:"status" binding:"omitempty,oneof=draft published scheduled"`
	BatchBaseTime        string `json:"batch_base_time" binding:"omitempty"`
	TitleIndex           int    `json:"title_index" binding:"min=0,max=999"`
	// Optional hints for {{content}} / {{meta_description}} in the bulk template.
	ContentPlaceholder         string `json:"content_placeholder" binding:"omitempty,max=4000"`
	MetaDescriptionPlaceholder string `json:"meta_description_placeholder" binding:"omitempty,max=2000"`
	LegacyIsiPlaceholder       string `json:"isi_placeholder" binding:"omitempty,max=4000"`
	LegacyDeskripsiPlaceholder string `json:"deskripsi_placeholder" binding:"omitempty,max=2000"`
	// CoverImageFromFeeds when true (default), tries RSS/Atom feeds configured for this category in scraper settings
	// and sets the post cover image from the best title-matching item.
	CoverImageFromFeeds *bool  `json:"cover_image_from_feeds,omitempty"`
	CoverImageMode      string `json:"cover_image_mode" binding:"omitempty,oneof=template generate_image web_search"`
	// CoverWebSearchSource limits web_search cover to one provider; empty/auto uses enabled providers in priority order.
	CoverWebSearchSource string `json:"cover_web_search_source" binding:"omitempty,oneof=auto openverse wikimedia unsplash pixabay pexels wordpress"`
	ImagePrompt          string `json:"image_prompt" binding:"omitempty,max=4000"`
	ImageToContent       bool   `json:"image_to_content_enabled"`
}
