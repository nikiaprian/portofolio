package entity

import (
	"path"
	"strings"
	"time"
)

const (
	HomeLayoutDefault               = "default"
	HomeLayoutDetik                 = "detik"
	HomeLayoutNeo                   = "neo"
	HomeLayoutTirtayasa             = "tirtayasa"
	NotFoundRedirectHome            = "home"
	NotFoundRedirectRandom          = "random"
	SourceDisclaimerDefault         = "default"
	SourceDisclaimerCustom          = "custom"
	ImageCaptionModeDefault         = "default"
	ImageCaptionModeCustom          = "custom"
	DefaultImageCaptionTemplate     = "Foto: Ilustrasi {{title}}."
	ArticleSchemaTypeNewsArticle    = "NewsArticle"
	ArticleSchemaTypeArticle        = "Article"
	DefaultSourceDisclaimerTemplate = "Sumber: {{source}}"
)

// SourceDisclaimerAdmitsAutomaticRewrite is the public sentence that says an
// article was rewritten automatically. That sentence stays off the site.
func SourceDisclaimerAdmitsAutomaticRewrite(tpl string) bool {
	t := strings.ToLower(tpl)
	if strings.Contains(t, "ditulis ulang") {
		return true
	}
	return strings.Contains(t, "otomatis") && strings.Contains(t, "ai")
}

const (
	DefaultSidebarListLimit         = 5
	MinSidebarListLimit             = 1
	MaxSidebarListLimit             = 20
	DefaultAlsoReadMaxInserts       = 6
	MinAlsoReadMaxInserts           = 1
	MaxAlsoReadMaxInserts           = 20
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
	case HomeLayoutTirtayasa:
		return HomeLayoutTirtayasa
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
	return SiteSetting{
		SiteName:                       "Tirtayasa",
		FooterCopy:                     "",
		Tagline:                        "Panduan layanan warga lewat jalur resmi.",
		LogoURL:                        "/static/images/logo.png",
		FaviconURL:                     "/static/images/favicon.ico",
		TextPrimary:                    "#1c1915",
		TextSecondary:                  "#5c564c",
		BackgroundPrimary:              "#ffffff",
		BackgroundSecondary:            "#ffffff",
		PrimaryButton:                  "#0f3d3e",
		SecondaryButton:                "#8a8175",
		AccentColor:                    "#0f3d3e",
		BorderColor:                    "#e4d9c8",
		FontFamily:                     "system-ui, -apple-system, \"Segoe UI\", Roboto, Ubuntu, \"Helvetica Neue\", sans-serif",
		ContentFontSize:                "17px",
		SidebarFontSize:                "14px",
		SidebarListLimit:               DefaultSidebarListLimit,
		SidebarListLimitPopular:        DefaultSidebarListLimit,
		SidebarListLimitLatest:         DefaultSidebarListLimit,
		ImageCaptionEnabled:            false,
		ImageCaptionMode:               ImageCaptionModeDefault,
		ImageCaptionTemplate:           DefaultImageCaptionTemplate,
		ImageResizeEnabled:             true,
		ImgResizeEnabled:               false,
		ImgResizeWidth:                 "",
		ImgResizeHeight:                "",
		ImgResizeQuality:               "",
		ImageResizeWidth:               "",
		ImageResizeQuality:             "",
		StoredImageMaxWidth:            "",
		StoredImageQuality:             "",
		AvatarResizeWidth:              "",
		AvatarResizeQuality:            "",
		NotFoundRedirectMode:           NotFoundRedirectHome,
		DefaultTitle:                   "Tirtayasa",
		DefaultDesc:                    "Cara mengurus bansos, BPJS, SKCK, NISN, dan urusan kerja tanpa calo, dengan tautan ke situs pemerintah.",
		DefaultImage:                   "",
		DefaultKeywords:                "bansos, BPJS, SKCK, NISN, perjanjian kerja, layanan warga",
		HeaderScript:                   "",
		FooterScript:                   "",
		AdsTop:                         "",
		AdsCenter:                      "",
		AdsBottom:                      "",
		AdsLazyLoadEnabled:             false,
		AdsTxt:                         "",
		RobotsTxt:                      "",
		ArticleAdsJSON:                 "",
		HomepageLayout:                 HomeLayoutTirtayasa,
		ColorTheme:                     ColorThemeCustom,
		SocialFacebookURL:              "",
		SocialXURL:                     "",
		SocialInstagramURL:             "",
		SocialYouTubeURL:               "",
		SocialTikTokURL:                "",
		SocialRSSURL:                   "",
		LoginURL:                       "",
		ArticleSlugMode:                ArticleSlugModeDirect,
		ArticleSchemaType:              ArticleSchemaTypeArticle,
		ScraperAutoTag:                 true,
		AutoHrefEnabled:                false,
		AlsoReadEnabled:                true,
		AlsoReadMaxInserts:             DefaultAlsoReadMaxInserts,
		PostDateRefreshEnabled:         false,
		PostDateRefreshDays:            30,
		PostDateIntervalMins:           30,
		PostDateUrlRecycleEnabled:      false,
		PostDateUrlRecycleIntervalMins: 0,
		SourceDisclaimerMode:           SourceDisclaimerDefault,
		SourceDisclaimerTemplate:       DefaultSourceDisclaimerTemplate,
	}
}

// EffectiveLoginHref returns the URL browsers and redirects should use for signing in.
// Empty raw means "/login". Relative paths must start with "/"; leading slash form only.
// http(s) URLs are returned unchanged. Unsafe or invalid values fall back to "/login".
func EffectiveLoginHref(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "/login"
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	if !strings.HasPrefix(s, "/") || strings.ContainsAny(s, " \t\n\r") {
		return "/login"
	}
	clean := path.Clean(s)
	if clean == "." || clean == "" {
		return "/login"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	pfx := strings.ToLower(clean)
	if strings.HasPrefix(pfx, "/admin") || strings.HasPrefix(pfx, "/api/") || strings.HasPrefix(pfx, "/static/") {
		return "/login"
	}
	return clean
}

// MultiSegmentLoginRoutePaths returns internal paths with 2+ URL segments (e.g. /akun/masuk) that must be
// registered explicitly on the router. Single-segment paths (e.g. /masuk) are handled in the /:slug handler.
func MultiSegmentLoginRoutePaths(raw string) []string {
	h := EffectiveLoginHref(raw)
	low := strings.ToLower(h)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return nil
	}
	if h == "/login" {
		return nil
	}
	inner := strings.Trim(h, "/")
	if inner == "" || !strings.Contains(inner, "/") {
		return nil
	}
	return []string{"/" + inner}
}

// SiteSetting stores general site settings (branding, URLs, etc.).
type SiteSetting struct {
	ID                  uint   `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	SiteName            string `gorm:"size:150;not null" json:"site_name"`
	FooterCopy          string `gorm:"size:255" json:"footer_copy"`
	Tagline             string `gorm:"size:255" json:"tagline"`
	LogoURL             string `gorm:"size:500" json:"logo_url"`
	FaviconURL          string `gorm:"size:500" json:"favicon_url"`
	TextPrimary         string `gorm:"size:20" json:"text_primary"`
	TextSecondary       string `gorm:"size:20" json:"text_secondary"`
	BackgroundPrimary   string `gorm:"size:20" json:"background_primary"`
	BackgroundSecondary string `gorm:"size:20" json:"background_secondary"`
	PrimaryButton       string `gorm:"size:20" json:"primary_button"`
	SecondaryButton     string `gorm:"size:20" json:"secondary_button"`
	AccentColor         string `gorm:"size:20" json:"accent_color"`
	BorderColor         string `gorm:"size:20" json:"border_color"`
	FontFamily          string `gorm:"size:255" json:"font_family"`
	ContentFontSize     string `gorm:"size:12" json:"content_font_size"`
	SidebarFontSize     string `gorm:"size:12" json:"sidebar_font_size"`
	// SidebarListLimit: legacy single limit (fallback when popular/latest unset).
	SidebarListLimit int `gorm:"column:sidebar_list_limit;not null;default:5" json:"sidebar_list_limit"`
	// SidebarListLimitPopular: atas — Populer (default), Berita terpopuler (detik), Terpopuler (neo).
	SidebarListLimitPopular int `gorm:"column:sidebar_list_limit_popular;not null;default:5" json:"sidebar_list_limit_popular"`
	// SidebarListLimitLatest: bawah — Terkini (default/neo), Rekomendasi (detik).
	SidebarListLimitLatest int  `gorm:"column:sidebar_list_limit_latest;not null;default:5" json:"sidebar_list_limit_latest"`
	ImageCaptionEnabled    bool `gorm:"column:image_caption_enabled;not null;default:false" json:"image_caption_enabled"`
	// ImageCaptionMode: default = per image_origin; custom = satu template untuk semua cover.
	ImageCaptionMode string `gorm:"column:image_caption_mode;size:20;not null;default:default" json:"image_caption_mode"`
	// ImageCaptionTemplate: {{title}}, {{provider}}, {{source}}, {{credit}} saat mode=custom.
	ImageCaptionTemplate string `gorm:"column:image_caption_template;type:text" json:"image_caption_template"`
	// ImageResizeEnabled: Cloudflare /cdn-cgi/image/ on list thumbnails (client-side rewrite).
	ImageResizeEnabled bool `gorm:"not null;default:true" json:"image_resize_enabled"`
	// ImgResizeEnabled: self-hosted /img proxy for thumbnails, avatars, and article body images.
	ImgResizeEnabled bool `gorm:"column:img_resize_enabled;not null;default:false" json:"img_resize_enabled"`
	// ImgResizeWidth: optional max width cap (px) for /img; actual width is set in the browser.
	ImgResizeWidth string `gorm:"column:img_resize_width;size:5" json:"img_resize_width"`
	// ImgResizeHeight: optional max height cap (px) for /img; actual height is set in the browser.
	ImgResizeHeight string `gorm:"column:img_resize_height;size:5" json:"img_resize_height"`
	// ImgResizeQuality: lossy WebP quality for /img proxy (posts, thumbnails, article body).
	ImgResizeQuality string `gorm:"column:img_resize_quality;size:3" json:"img_resize_quality"`
	// ImageResizeWidth/Quality: Cloudflare /cdn-cgi/image/ delivery (zone must be on Cloudflare).
	ImageResizeWidth   string `gorm:"size:5" json:"image_resize_width"`
	ImageResizeQuality string `gorm:"size:3" json:"image_resize_quality"`
	// StoredImageMaxWidth caps the longest edge (px) for AI/scraper post images saved to disk/R2 (independent of CDN).
	StoredImageMaxWidth  string `gorm:"size:5" json:"stored_image_max_width"`
	StoredImageQuality   string `gorm:"size:3" json:"stored_image_quality"`
	AvatarResizeWidth    string `gorm:"size:5" json:"avatar_resize_width"`
	AvatarResizeQuality  string `gorm:"size:3" json:"avatar_resize_quality"`
	NotFoundRedirectMode string `gorm:"size:20;not null;default:home" json:"not_found_redirect_mode"`
	DefaultTitle         string `gorm:"size:255;not null" json:"default_title"`
	DefaultDesc          string `gorm:"size:500" json:"default_description"`
	DefaultImage         string `gorm:"size:500" json:"default_image"`
	DefaultKeywords      string `gorm:"size:500" json:"default_keywords"`
	HeaderScript         string `gorm:"type:text" json:"header_script"`
	FooterScript         string `gorm:"type:text" json:"footer_script"`
	AdsTop               string `gorm:"type:text" json:"ads_top"`
	AdsCenter            string `gorm:"type:text" json:"ads_center"`
	AdsBottom            string `gorm:"type:text" json:"ads_bottom"`
	// AdsLazyLoadEnabled when true defers AdSense push until each .site-ad nears the viewport.
	AdsLazyLoadEnabled bool   `gorm:"column:ads_lazy_load_enabled;not null;default:false" json:"ads_lazy_load_enabled"`
	AdsTxt             string `gorm:"type:text" json:"ads_txt"`
	RobotsTxt          string `gorm:"type:text" json:"robots_txt"`
	// ArticleAdsJSON stores in-article ads as a JSON array, e.g. [{"position":25,"content":"<script>...</script>"}].
	// Position is an integer 1-99 representing the percentage of article paragraphs to render after.
	ArticleAdsJSON string `gorm:"column:article_ads_json;type:text" json:"article_ads_json"`
	// JSON blobs shared with the portfolio schema. Runtime fields below stay in sync via Pack/Unpack.
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
	HomepageLayout         string `gorm:"size:20;not null;default:default" json:"homepage_layout"`
	ColorTheme             string `gorm:"-" json:"color_theme"`
	SocialFacebookURL      string `gorm:"size:500" json:"social_facebook_url"`
	SocialXURL             string `gorm:"size:500" json:"social_x_url"`
	SocialInstagramURL     string `gorm:"size:500" json:"social_instagram_url"`
	SocialYouTubeURL       string `gorm:"size:500" json:"social_youtube_url"`
	SocialTikTokURL        string `gorm:"size:500" json:"social_tiktok_url"`
	SocialRSSURL           string `gorm:"size:500" json:"social_rss_url"`
	LoginURL               string `gorm:"size:500" json:"login_url"`
	ArticleSlugMode        string `gorm:"size:32;not null;default:direct" json:"article_slug_mode"`
	ArticleSchemaType      string `gorm:"column:article_schema_type;size:30;not null;default:Article" json:"article_schema_type"`
	// ScraperAutoTag when false disables scraper tag inference, in-body tag links, and post tag associations (all categories and bulk runs).
	ScraperAutoTag bool `gorm:"column:scraper_auto_tag;not null;default:true" json:"scraper_auto_tag"`
	// AutoHrefEnabled when true enables auto-linking of category names and article titles in post contents.
	AutoHrefEnabled bool `gorm:"column:auto_href_enabled;not null;default:false" json:"auto_href_enabled"`
	// AlsoReadEnabled when false strips in-body "Baca juga" blocks on save and hides them on public render.
	AlsoReadEnabled bool `gorm:"column:also_read_enabled;not null;default:true" json:"also_read_enabled"`
	// AlsoReadMaxInserts caps how many "Baca juga" boxes are injected per published article (1–20).
	AlsoReadMaxInserts int `gorm:"column:also_read_max_inserts;not null;default:6" json:"also_read_max_inserts"`
	// AlsoReadParagraphInterval is kept so saves round-trip the shared JSON blob. This app does not apply the interval itself.
	AlsoReadParagraphInterval int `gorm:"-" json:"also_read_paragraph_interval"`
	// PostDateRefreshEnabled when false disables the background job that refreshes old published post dates.
	PostDateRefreshEnabled bool `gorm:"column:post_date_refresh_enabled;not null;default:false" json:"post_date_refresh_enabled"`
	// PostDateRefreshDays: when >0, published posts older than N days get their updated_at refreshed by background worker.
	PostDateRefreshDays int `gorm:"column:post_date_refresh_days;not null;default:30" json:"post_date_refresh_days"`
	// PostDateIntervalMins: per-post delay (minutes) when refreshing multiple post dates in one cycle.
	PostDateIntervalMins int `gorm:"column:post_date_interval_mins;not null;default:30" json:"post_date_interval_mins"`
	// PostDateUrlRecycleEnabled gates the in-memory queue worker that bumps published posts' updated_at for submitted URLs.
	PostDateUrlRecycleEnabled bool `gorm:"column:post_date_url_recycle_enabled;not null;default:false" json:"post_date_url_recycle_enabled"`
	// PostDateUrlRecycleIntervalMins: minimum minutes between processing consecutive queued URLs (0 = each worker minute).
	PostDateUrlRecycleIntervalMins int `gorm:"column:post_date_url_recycle_interval_mins;not null;default:0" json:"post_date_url_recycle_interval_mins"`
	// SourceDisclaimerMode controls disclaimer rendering mode on post page: default/custom.
	SourceDisclaimerMode string `gorm:"column:source_disclaimer_mode;size:20;not null;default:default" json:"source_disclaimer_mode"`
	// SourceDisclaimerTemplate supports {{source}} placeholder when mode=custom.
	SourceDisclaimerTemplate string    `gorm:"column:source_disclaimer_template;type:text" json:"source_disclaimer_template"`
	CreatedAt                time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt                time.Time `json:"updated_at" swaggerignore:"true"`
}
