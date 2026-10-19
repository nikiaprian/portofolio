package entity

import (
	"strings"
	"time"
)

const (
	HomeLayoutDefault               = "default"
	HomeLayoutDetik                 = "detik"
	HomeLayoutNeo                   = "neo"
	NotFoundRedirectHome            = "home"
	NotFoundRedirectRandom          = "random"
	SourceDisclaimerDefault         = "default"
	SourceDisclaimerCustom          = "custom"
	ImageCaptionModeDefault         = "default"
	ImageCaptionModeCustom          = "custom"
	DefaultImageCaptionTemplate     = "Foto: Ilustrasi {{title}}."
	ArticleSchemaTypeNewsArticle    = "NewsArticle"
	ArticleSchemaTypeArticle        = "Article"
	DefaultSourceDisclaimerTemplate = "Disclaimer:\nArtikel ini ditulis ulang secara otomatis oleh AI berdasarkan sumber {{source}} tanpa mengubah fakta pada artikel asli."
	DefaultSidebarListLimit         = 5
	MinSidebarListLimit             = 1
	MaxSidebarListLimit             = 20
	DefaultAlsoReadMaxInserts        = 6
	MinAlsoReadMaxInserts            = 1
	MaxAlsoReadMaxInserts            = 20
	DefaultAlsoReadParagraphInterval = 3
	MinAlsoReadParagraphInterval     = 1
	MaxAlsoReadParagraphInterval     = 20
)

// EffectiveAlsoReadMaxInserts clamps in-body "Baca juga" slots per published article.
func EffectiveAlsoReadMaxInserts(n int) int {
	if n <= 0 {
		return DefaultAlsoReadMaxInserts
	}
	if n < MinAlsoReadMaxInserts {
		return MinAlsoReadMaxInserts
	}
	if n > MaxAlsoReadMaxInserts {
		return MaxAlsoReadMaxInserts
	}
	return n
}

// EffectiveAlsoReadParagraphInterval clamps how many non-empty <p> blocks between each "Baca juga" insert.
func EffectiveAlsoReadParagraphInterval(n int) int {
	if n <= 0 {
		return DefaultAlsoReadParagraphInterval
	}
	if n < MinAlsoReadParagraphInterval {
		return MinAlsoReadParagraphInterval
	}
	if n > MaxAlsoReadParagraphInterval {
		return MaxAlsoReadParagraphInterval
	}
	return n
}

// EffectiveSidebarListLimit clamps sidebar list sizes (Populer, Terkini, Berita terpopuler, Rekomendasi).
func EffectiveSidebarListLimit(n int) int {
	if n <= 0 {
		return DefaultSidebarListLimit
	}
	if n < MinSidebarListLimit {
		return MinSidebarListLimit
	}
	if n > MaxSidebarListLimit {
		return MaxSidebarListLimit
	}
	return n
}

// NormalizeSidebarListLimits fills popular/latest from legacy sidebar_list_limit when unset.
func NormalizeSidebarListLimits(s *SiteSetting) {
	if s == nil {
		return
	}
	legacy := EffectiveSidebarListLimit(s.SidebarListLimit)
	if s.SidebarListLimitPopular <= 0 {
		s.SidebarListLimitPopular = legacy
	} else {
		s.SidebarListLimitPopular = EffectiveSidebarListLimit(s.SidebarListLimitPopular)
	}
	if s.SidebarListLimitLatest <= 0 {
		s.SidebarListLimitLatest = legacy
	} else {
		s.SidebarListLimitLatest = EffectiveSidebarListLimit(s.SidebarListLimitLatest)
	}
}

func NormalizeHomeLayout(v string) string {
	switch v {
	case HomeLayoutDetik:
		return HomeLayoutDetik
	case HomeLayoutNeo:
		return HomeLayoutNeo
	default:
		return HomeLayoutDefault
	}
}

func NormalizeNotFoundRedirectMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case NotFoundRedirectRandom:
		return NotFoundRedirectRandom
	default:
		return NotFoundRedirectHome
	}
}

func NormalizeArticleSchemaType(v string) string {
	switch strings.TrimSpace(v) {
	case ArticleSchemaTypeNewsArticle:
		return ArticleSchemaTypeNewsArticle
	default:
		return ArticleSchemaTypeArticle
	}
}

// DefaultSiteSetting returns baseline values when no row exists in site_settings yet.
func DefaultSiteSetting() SiteSetting {
	colors := SiteColorPreset(ColorThemeDefault)
	d := SiteSetting{
		SiteName:   "Portfolio",
		FooterCopy: "",
		Tagline:    "",
		LogoURL:    "",
		FaviconURL: "/static/images/favicon.ico",

		ColorTheme:          ColorThemeDefault,
		TextPrimary:         colors.TextPrimary,
		TextSecondary:       colors.TextSecondary,
		BackgroundPrimary:   colors.BackgroundPrimary,
		BackgroundSecondary: colors.BackgroundSecondary,
		PrimaryButton:       colors.PrimaryButton,
		SecondaryButton:     colors.SecondaryButton,
		AccentColor:         colors.AccentColor,
		BorderColor:         colors.BorderColor,

		FontFamily:              `system-ui, -apple-system, "Segoe UI", Roboto, Ubuntu, "Helvetica Neue", sans-serif`,
		ContentFontSize:         "17px",
		SidebarFontSize:         "14px",
		SidebarListLimit:        DefaultSidebarListLimit,
		SidebarListLimitPopular: DefaultSidebarListLimit,
		SidebarListLimitLatest:  DefaultSidebarListLimit,

		ImageCaptionEnabled:  false,
		ImageCaptionMode:     ImageCaptionModeDefault,
		ImageCaptionTemplate: DefaultImageCaptionTemplate,

		ImageResizeEnabled: true,
		ImgResizeEnabled:   false,

		NotFoundRedirectMode: NotFoundRedirectHome,
		DefaultTitle:         "Portfolio",

		HomepageLayout: HomeLayoutDefault,

		ArticleSlugMode:   ArticleSlugModeDirect,
		ArticleSchemaType: ArticleSchemaTypeArticle,
		ScraperAutoTag:    true,
		AutoHrefEnabled:   false,

		AlsoReadEnabled:           true,
		AlsoReadMaxInserts:        DefaultAlsoReadMaxInserts,
		AlsoReadParagraphInterval: DefaultAlsoReadParagraphInterval,

		PostDateRefreshEnabled: false,
		PostDateRefreshDays:    30,
		PostDateIntervalMins:   30,

		SourceDisclaimerMode:     SourceDisclaimerDefault,
		SourceDisclaimerTemplate: DefaultSourceDisclaimerTemplate,
	}
	PackSiteSettingJSON(&d)
	return d
}

// SiteSetting stores general site settings (branding, URLs, etc.).
type SiteSetting struct {
	ID uint `gorm:"primaryKey" json:"id" swaggerignore:"true"`

	// --- Flat branding columns ---
	SiteName   string `gorm:"size:150;not null" json:"site_name"`
	FooterCopy string `gorm:"size:255" json:"footer_copy"`
	Tagline    string `gorm:"size:255" json:"tagline"`
	LogoURL    string `gorm:"size:500" json:"logo_url"`
	FaviconURL string `gorm:"size:500" json:"favicon_url"`

	// --- JSON blob columns (Stored* ↔ site_settings.* JSON text columns) ---
	StoredColorTheme       string `gorm:"column:color_theme;type:text" json:"-"`
	StoredAdsScript        string `gorm:"column:ads_script;type:text" json:"-"`
	StoredImgResizeSelf    string `gorm:"column:img_resize_self;type:text" json:"-"`
	StoredImgResizeCDN     string `gorm:"column:img_resize_cdn;type:text" json:"-"`
	StoredSocialLinks      string `gorm:"column:social_links;type:text" json:"-"`
	StoredTypography       string `gorm:"column:typography;type:text" json:"-"`
	StoredSidebarConfig    string `gorm:"column:sidebar_config;type:text" json:"-"`
	StoredImageCaption     string `gorm:"column:image_caption;type:text" json:"-"`
	StoredDefaultMeta      string `gorm:"column:default_meta;type:text" json:"-"`
	StoredSourceDisclaimer string `gorm:"column:source_disclaimer;type:text" json:"-"`
	StoredAlsoRead         string `gorm:"column:also_read;type:text" json:"-"`
	StoredPostDateConfig   string `gorm:"column:post_date_config;type:text" json:"-"`
	StoredSeoFiles         string `gorm:"column:seo_files;type:text" json:"-"`

	// --- Runtime: color theme (StoredColorTheme) ---
	ColorTheme          string `gorm:"-" json:"color_theme"`
	TextPrimary         string `gorm:"-" json:"text_primary"`
	TextSecondary       string `gorm:"-" json:"text_secondary"`
	BackgroundPrimary   string `gorm:"-" json:"background_primary"`
	BackgroundSecondary string `gorm:"-" json:"background_secondary"`
	PrimaryButton       string `gorm:"-" json:"primary_button"`
	SecondaryButton     string `gorm:"-" json:"secondary_button"`
	AccentColor         string `gorm:"-" json:"accent_color"`
	BorderColor         string `gorm:"-" json:"border_color"`

	// --- Runtime: typography (StoredTypography) ---
	FontFamily      string `gorm:"-" json:"font_family"`
	ContentFontSize string `gorm:"-" json:"content_font_size"`
	SidebarFontSize string `gorm:"-" json:"sidebar_font_size"`

	// --- Runtime: sidebar (StoredSidebarConfig) ---
	SidebarListLimit        int `gorm:"-" json:"sidebar_list_limit"`
	SidebarListLimitPopular int `gorm:"-" json:"sidebar_list_limit_popular"`
	SidebarListLimitLatest  int `gorm:"-" json:"sidebar_list_limit_latest"`

	// --- Runtime: image caption (StoredImageCaption) ---
	ImageCaptionEnabled  bool   `gorm:"-" json:"image_caption_enabled"`
	ImageCaptionMode     string `gorm:"-" json:"image_caption_mode"`
	ImageCaptionTemplate string `gorm:"-" json:"image_caption_template"`

	// --- Runtime: image resize (StoredImgResizeSelf, StoredImgResizeCDN) ---
	ImageResizeEnabled  bool   `gorm:"-" json:"image_resize_enabled"`
	ImgResizeEnabled    bool   `gorm:"-" json:"img_resize_enabled"`
	ImgResizeWidth      string `gorm:"-" json:"img_resize_width"`
	ImgResizeHeight     string `gorm:"-" json:"img_resize_height"`
	ImgResizeQuality    string `gorm:"-" json:"img_resize_quality"`
	ImageResizeWidth    string `gorm:"-" json:"image_resize_width"`
	ImageResizeQuality  string `gorm:"-" json:"image_resize_quality"`
	StoredImageMaxWidth string `gorm:"-" json:"stored_image_max_width"`
	StoredImageQuality  string `gorm:"-" json:"stored_image_quality"`
	AvatarResizeWidth   string `gorm:"-" json:"avatar_resize_width"`
	AvatarResizeQuality string `gorm:"-" json:"avatar_resize_quality"`

	// --- Runtime: ads & scripts (StoredAdsScript) ---
	HeaderScript       string `gorm:"-" json:"header_script"`
	FooterScript       string `gorm:"-" json:"footer_script"`
	AdsTop             string `gorm:"-" json:"ads_top"`
	AdsCenter          string `gorm:"-" json:"ads_center"`
	AdsBottom          string `gorm:"-" json:"ads_bottom"`
	AdsLazyLoadEnabled bool   `gorm:"-" json:"ads_lazy_load_enabled"`
	ArticleAdsJSON     string `gorm:"-" json:"article_ads_json"`

	// --- Runtime: default meta (StoredDefaultMeta) ---
	DefaultTitle    string `gorm:"-" json:"default_title"`
	DefaultDesc     string `gorm:"-" json:"default_description"`
	DefaultImage    string `gorm:"-" json:"default_image"`
	DefaultKeywords string `gorm:"-" json:"default_keywords"`

	// --- Runtime: social links (StoredSocialLinks) ---
	SocialFacebookURL  string `gorm:"-" json:"social_facebook_url"`
	SocialXURL         string `gorm:"-" json:"social_x_url"`
	SocialInstagramURL string `gorm:"-" json:"social_instagram_url"`
	SocialYouTubeURL   string `gorm:"-" json:"social_youtube_url"`
	SocialTikTokURL    string `gorm:"-" json:"social_tiktok_url"`
	SocialRSSURL       string `gorm:"-" json:"social_rss_url"`

	// --- Flat behavior columns ---
	NotFoundRedirectMode       string `gorm:"size:20;not null;default:home" json:"not_found_redirect_mode"`
	HomepageLayout             string `gorm:"size:20;not null;default:default" json:"homepage_layout"`
	LoginURL                   string `gorm:"size:500" json:"login_url"`
	ArticleSlugMode            string `gorm:"size:32;not null;default:direct" json:"article_slug_mode"`
	ArticleSchemaType          string `gorm:"column:article_schema_type;size:30;not null;default:Article" json:"article_schema_type"`
	ScraperAutoTag             bool   `gorm:"column:scraper_auto_tag;not null;default:true" json:"scraper_auto_tag"`
	AutoHrefEnabled            bool   `gorm:"column:auto_href_enabled;not null;default:false" json:"auto_href_enabled"`
	FrontendRoutesDisabled     bool   `gorm:"column:frontend_routes_disabled;not null;default:false" json:"frontend_routes_disabled"`
	AdminLoginFrontendDisabled bool   `gorm:"column:admin_login_frontend_disabled;not null;default:false" json:"admin_login_frontend_disabled"`

	// --- Runtime: also read (StoredAlsoRead) ---
	AlsoReadEnabled           bool `gorm:"-" json:"also_read_enabled"`
	AlsoReadMaxInserts        int  `gorm:"-" json:"also_read_max_inserts"`
	AlsoReadParagraphInterval int  `gorm:"-" json:"also_read_paragraph_interval"`

	// --- Runtime: post date worker (StoredPostDateConfig) ---
	PostDateRefreshEnabled         bool `gorm:"-" json:"post_date_refresh_enabled"`
	PostDateRefreshDays            int  `gorm:"-" json:"post_date_refresh_days"`
	PostDateIntervalMins           int  `gorm:"-" json:"post_date_interval_mins"`
	PostDateUrlRecycleEnabled      bool `gorm:"-" json:"post_date_url_recycle_enabled"`
	PostDateUrlRecycleIntervalMins int  `gorm:"-" json:"post_date_url_recycle_interval_mins"`

	// --- Runtime: source disclaimer (StoredSourceDisclaimer) ---
	SourceDisclaimerMode     string `gorm:"-" json:"source_disclaimer_mode"`
	SourceDisclaimerTemplate string `gorm:"-" json:"source_disclaimer_template"`

	// --- Runtime: SEO files (StoredSeoFiles) ---
	AdsTxt    string `gorm:"-" json:"ads_txt"`
	RobotsTxt string `gorm:"-" json:"robots_txt"`

	CreatedAt time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt time.Time `json:"updated_at" swaggerignore:"true"`
}
