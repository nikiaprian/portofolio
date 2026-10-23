package dto

// UpdateSiteConnectionRequest is the body for PUT /admin/site-connections.
type UpdateSiteConnectionRequest struct {
	Ahrefs string `json:"ahrefs" binding:"max=512"`
	Google string `json:"google" binding:"max=512"`
	Bing   string `json:"bing" binding:"max=512"`
	Yandex string `json:"yandex" binding:"max=512"`

	AutoPostEnabled  bool   `json:"auto_post_enabled"`
	AutoPostTemplate string `json:"auto_post_template" binding:"max=20000"`

	FacebookEnabled      bool   `json:"facebook_enabled"`
	FacebookClientID     string `json:"facebook_client_id" binding:"max=255"`
	FacebookClientSecret string `json:"facebook_client_secret" binding:"max=255"`
	FacebookTokenURL     string `json:"facebook_token_url" binding:"max=500"`
	FacebookPublishURL   string `json:"facebook_publish_url" binding:"max=500"`
	FacebookScope        string `json:"facebook_scope" binding:"max=500"`
	FacebookAccountID    string `json:"facebook_account_id" binding:"max=255"`

	XEnabled      bool   `json:"x_enabled"`
	XClientID     string `json:"x_client_id" binding:"max=255"`
	XClientSecret string `json:"x_client_secret" binding:"max=255"`
	XTokenURL     string `json:"x_token_url" binding:"max=500"`
	XPublishURL   string `json:"x_publish_url" binding:"max=500"`
	XScope        string `json:"x_scope" binding:"max=500"`
	XAccountID    string `json:"x_account_id" binding:"max=255"`

	TiktokEnabled      bool   `json:"tiktok_enabled"`
	TiktokClientID     string `json:"tiktok_client_id" binding:"max=255"`
	TiktokClientSecret string `json:"tiktok_client_secret" binding:"max=255"`
	TiktokTokenURL     string `json:"tiktok_token_url" binding:"max=500"`
	TiktokPublishURL   string `json:"tiktok_publish_url" binding:"max=500"`
	TiktokScope        string `json:"tiktok_scope" binding:"max=500"`
	TiktokAccountID    string `json:"tiktok_account_id" binding:"max=255"`

	InstagramEnabled      bool   `json:"instagram_enabled"`
	InstagramClientID     string `json:"instagram_client_id" binding:"max=255"`
	InstagramClientSecret string `json:"instagram_client_secret" binding:"max=255"`
	InstagramTokenURL     string `json:"instagram_token_url" binding:"max=500"`
	InstagramPublishURL   string `json:"instagram_publish_url" binding:"max=500"`
	InstagramScope        string `json:"instagram_scope" binding:"max=500"`
	InstagramAccountID    string `json:"instagram_account_id" binding:"max=255"`
}

// TestAutoPostRequest triggers a manual webhook test without publishing a real post.
type TestAutoPostRequest struct {
	Title   string  `json:"title" binding:"omitempty,min=3,max=200"`
	Slug    string  `json:"slug" binding:"omitempty,min=3,max=255,slug"`
	Content string  `json:"content" binding:"omitempty,max=5000"`
	Image   *string `json:"image" binding:"omitempty,max=500"`
}
