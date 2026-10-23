package dto

// ScraperSettingResponse is returned by GET/PUT /admin/scraper-setting.
type ScraperSettingResponse struct {
	ID                         uint                `json:"id" example:"1"`
	Enabled                    bool                `json:"enabled" example:"false"`
	IntervalMins               int                 `json:"interval_mins" example:"5"`
	PauseBetweenCategoriesMins int                 `json:"pause_between_categories_mins" example:"0"`
	ScraperAIProvider          string              `json:"scraper_ai_provider" example:"auto"`
	CategoryFeeds              []CategoryFeedEntry `json:"category_feeds"`
}

// CategoryFeedEntry is one category's feed configuration.
type CategoryFeedEntry struct {
	CategorySlug   string `json:"category_slug" example:"nasional"`
	CategoryName   string `json:"category_name" example:"Nasional"`
	FeedURLs       string `json:"feed_urls" example:"https://www.example.com/rss"`
	Enabled        bool   `json:"enabled" example:"true"`
	IncludeSource  bool   `json:"include_source" example:"true"`
	ArticleStatus  string `json:"article_status" example:"published"`
	ImageMode         string `json:"image_mode" example:"scrape"`
	WebSearchSource   string `json:"web_search_source" example:"auto"`
	ImageToContent    bool   `json:"image_to_content" example:"false"`
	AuthorEmail    string `json:"author_email" example:"writer@example.com"`
}

// UpdateScraperSettingRequest is the body for PUT /admin/scraper-setting.
type UpdateScraperSettingRequest struct {
	Enabled                    bool                 `json:"enabled" example:"false"`
	IntervalMins               int                  `json:"interval_mins" binding:"min=1,max=1440" example:"5"`
	PauseBetweenCategoriesMins int                  `json:"pause_between_categories_mins" binding:"min=0,max=1440" example:"0"`
	ScraperAIProvider          string               `json:"scraper_ai_provider" binding:"omitempty,oneof=off auto gemini openai groq claude" example:"auto"`
	CategoryFeeds              []CategoryFeedUpdate `json:"category_feeds"` // omit or null to leave per-category rows unchanged
}

// CategoryFeedUpdate is a single category's feeds from the client.
type CategoryFeedUpdate struct {
	CategorySlug   string  `json:"category_slug" binding:"required,slug,max=150" example:"nasional"`
	FeedURLs       string  `json:"feed_urls" binding:"max=20000" example:"https://www.example.com/rss, https://www.example.com/feed"`
	Enabled        bool    `json:"enabled" example:"true"`
	IncludeSource  bool    `json:"include_source" example:"true"`
	ArticleStatus  *string `json:"article_status,omitempty" binding:"omitempty,oneof=draft published" example:"published"`
	ImageMode         *string `json:"image_mode,omitempty" binding:"omitempty,oneof=scrape generate generate_image web_search" example:"scrape"`
	WebSearchSource   *string `json:"web_search_source,omitempty" binding:"omitempty,oneof=auto openverse wikimedia unsplash pixabay pexels wordpress" example:"auto"`
	ImageToContent    *bool   `json:"image_to_content,omitempty" example:"false"`
	// AuthorEmail, when set, updates the per-category scraper author. Omit to leave unchanged.
	AuthorEmail *string `json:"author_email,omitempty" binding:"omitempty,email,max=255" example:"writer@example.com"`
}
