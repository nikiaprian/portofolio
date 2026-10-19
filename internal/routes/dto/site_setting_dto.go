package dto

// UpdateSiteSettingRequest is the body for PUT /admin/site-setting.
type UpdateSiteSettingRequest struct {
	SiteName                string `json:"site_name" binding:"required,min=1,max=150"`
	FooterCopy              string `json:"footer_copy" binding:"max=255"`
	Tagline                 string `json:"tagline" binding:"max=255"`
	LogoURL                 string `json:"logo_url" binding:"max=500"`
	FaviconURL              string `json:"favicon_url" binding:"max=500"`
	TextPrimary             string `json:"text_primary" binding:"max=20"`
	TextSecondary           string `json:"text_secondary" binding:"max=20"`
	BackgroundPrimary       string `json:"background_primary" binding:"max=20"`
	BackgroundSecondary     string `json:"background_secondary" binding:"max=20"`
	PrimaryButton           string `json:"primary_button" binding:"max=20"`
	SecondaryButton         string `json:"secondary_button" binding:"max=20"`
	AccentColor             string `json:"accent_color" binding:"max=20"`
	BorderColor             string `json:"border_color" binding:"max=20"`
	FontFamily              string `json:"font_family" binding:"max=255"`
	ContentFontSize         string `json:"content_font_size" binding:"max=12"`
	SidebarFontSize         string `json:"sidebar_font_size" binding:"max=12"`
	SidebarListLimit        int    `json:"sidebar_list_limit" binding:"omitempty,min=1,max=20"`
	SidebarListLimitPopular int    `json:"sidebar_list_limit_popular" binding:"omitempty,min=1,max=20"`
	SidebarListLimitLatest  int    `json:"sidebar_list_limit_latest" binding:"omitempty,min=1,max=20"`
	ImageCaptionEnabled     bool   `json:"image_caption_enabled"`
	ImageCaptionMode        string `json:"image_caption_mode" binding:"omitempty,oneof=default custom"`
	ImageCaptionTemplate    string `json:"image_caption_template" binding:"max=5000"`
	// ImgResizeEnabled: admin UI always sends explicit true/false on PUT /admin/site-setting.
	ImgResizeEnabled   bool   `json:"img_resize_enabled"`
	ImgResizeWidth     string `json:"img_resize_width" binding:"max=5"`
	ImgResizeHeight    string `json:"img_resize_height" binding:"max=5"`
	ImgResizeQuality   string `json:"img_resize_quality" binding:"max=3"`
	ImageResizeWidth   string `json:"image_resize_width" binding:"max=5"`
	ImageResizeQuality string `json:"image_resize_quality" binding:"max=3"`
	// ImageResizeEnabled omitempty: omit on old clients to preserve DB value; when set, toggles Cloudflare /cdn-cgi/image/ resize on list thumbnails.
	ImageResizeEnabled   *bool  `json:"image_resize_enabled,omitempty"`
	StoredImageMaxWidth  string `json:"stored_image_max_width" binding:"max=5"`
	StoredImageQuality   string `json:"stored_image_quality" binding:"max=3"`
	AvatarResizeWidth    string `json:"avatar_resize_width" binding:"max=5"`
	AvatarResizeQuality  string `json:"avatar_resize_quality" binding:"max=3"`
	NotFoundRedirectMode string `json:"not_found_redirect_mode" binding:"omitempty,oneof=home random"`
	DefaultTitle         string `json:"default_title" binding:"max=255"`
	DefaultDesc          string `json:"default_description" binding:"max=500"`
	DefaultImage         string `json:"default_image" binding:"max=500"`
	DefaultKeywords      string `json:"default_keywords" binding:"max=500"`
	HomepageLayout       string `json:"homepage_layout" binding:"omitempty,oneof=default detik neo"`
	ColorTheme           string `json:"color_theme" binding:"omitempty,oneof=default ocean forest sunset slate violet amber dark custom"`
	SocialFacebookURL    string `json:"social_facebook_url" binding:"max=500"`
	SocialXURL           string `json:"social_x_url" binding:"max=500"`
	SocialInstagramURL   string `json:"social_instagram_url" binding:"max=500"`
	SocialYouTubeURL     string `json:"social_youtube_url" binding:"max=500"`
	SocialTikTokURL      string `json:"social_tiktok_url" binding:"max=500"`
	SocialRSSURL         string `json:"social_rss_url" binding:"max=500"`
	LoginURL             string `json:"login_url" binding:"max=500"`
	// FrontendRoutesDisabled disables all public frontend SSR routes.
	// Pointer keeps backward compatibility with older admin UIs that omit this field.
	FrontendRoutesDisabled *bool `json:"frontend_routes_disabled,omitempty"`
	// AdminLoginFrontendDisabled disables SSR login and /admin pages.
	AdminLoginFrontendDisabled *bool  `json:"admin_login_frontend_disabled,omitempty"`
	ArticleSlugMode            string `json:"article_slug_mode" binding:"omitempty,max=32"`
	ArticleSchemaType          string `json:"article_schema_type" binding:"omitempty,oneof=NewsArticle Article"`
	// ScraperAutoTag omitempty: omit on old clients to preserve DB value; when set, controls scraper auto-tagging site-wide.
	ScraperAutoTag *bool `json:"scraper_auto_tag,omitempty"`
	// AutoHrefEnabled omitempty: omit on old clients to preserve DB value; when set, toggles in-body auto-link for categories and articles.
	AutoHrefEnabled *bool `json:"auto_href_enabled,omitempty"`
	// AlsoReadEnabled: admin UI always sends explicit true/false on PUT /admin/site-setting.
	AlsoReadEnabled bool `json:"also_read_enabled"`
	// AlsoReadMaxInserts: max in-body "Baca juga" boxes per article when also_read_enabled is true.
	AlsoReadMaxInserts int `json:"also_read_max_inserts" binding:"omitempty,min=1,max=20"`
	// AlsoReadParagraphInterval omitempty: omit on old admin JS to preserve DB value; when set, controls Baca juga spacing.
	AlsoReadParagraphInterval *int `json:"also_read_paragraph_interval,omitempty" binding:"omitempty,min=1,max=20"`
	// PostDateRefreshEnabled: admin UI always sends explicit true/false on PUT /admin/site-setting.
	PostDateRefreshEnabled bool `json:"post_date_refresh_enabled"`
	// PostDateRefreshDays: when >0, refresh updated_at for posts older than this many days.
	PostDateRefreshDays int `json:"post_date_refresh_days" binding:"omitempty,min=0,max=3650"`
	// PostDateIntervalMins: delay in minutes between refreshed posts in one run.
	PostDateIntervalMins int `json:"post_date_interval_mins" binding:"omitempty,min=0,max=1440"`
	// PostDateUrlRecycleEnabled: gate the in-memory queue worker that bumps matching published posts' updated_at.
	PostDateUrlRecycleEnabled bool `json:"post_date_url_recycle_enabled"`
	// PostDateUrlRecycleIntervalMins: minutes between each queued URL processed by the worker (0 = as often as the worker minute tick).
	PostDateUrlRecycleIntervalMins int `json:"post_date_url_recycle_interval_mins" binding:"omitempty,min=0,max=1440"`
	// SourceDisclaimerMode controls disclaimer rendering mode for source link on post page.
	SourceDisclaimerMode string `json:"source_disclaimer_mode" binding:"omitempty,oneof=default custom"`
	// SourceDisclaimerTemplate supports {{source}} placeholder when SourceDisclaimerMode=custom.
	SourceDisclaimerTemplate string `json:"source_disclaimer_template" binding:"max=5000"`
}

// PostDateUrlRecycleRunRequest is the body for POST /admin/site-setting/post-date-url-recycle/run.
// URLs is newline-separated; entries are appended to the in-memory queue (not persisted).
type PostDateUrlRecycleRunRequest struct {
	URLs string `json:"urls" binding:"max=65535"`
}

// UpdateSiteScriptsRequest is the body for PUT /admin/site-setting/scripts.
type UpdateSiteScriptsRequest struct {
	HeaderScript string `json:"header_script" binding:"max=65535"`
	FooterScript string `json:"footer_script" binding:"max=65535"`
	AdsTop       string `json:"ads_top" binding:"max=65535"`
	AdsCenter    string `json:"ads_center" binding:"max=65535"`
	AdsBottom    string `json:"ads_bottom" binding:"max=65535"`
	// AdsLazyLoadEnabled when set, toggles lazy viewport activation for .site-ad (omit to leave unchanged).
	AdsLazyLoadEnabled *bool            `json:"ads_lazy_load_enabled,omitempty"`
	AdsTxt             string           `json:"ads_txt" binding:"max=65535"`
	RobotsTxt          string           `json:"robots_txt" binding:"max=65535"`
	ArticleAds         []ArticleAdInput `json:"article_ads" binding:"omitempty,max=10,dive"`
}

// ArticleAdInput is a single in-article ad slot (paste into post body at a paragraph % position).
type ArticleAdInput struct {
	Position int    `json:"position" binding:"required,min=1,max=99"`
	Content  string `json:"content" binding:"required,max=65535"`
}
