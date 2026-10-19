package entity

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"
)

// SiteColorThemeJSON stores preset + custom color tokens in site_settings.color_theme.
type SiteColorThemeJSON struct {
	Preset              string `json:"preset"`
	TextPrimary         string `json:"text_primary"`
	TextSecondary       string `json:"text_secondary"`
	BackgroundPrimary   string `json:"background_primary"`
	BackgroundSecondary string `json:"background_secondary"`
	PrimaryButton       string `json:"primary_button"`
	SecondaryButton     string `json:"secondary_button"`
	AccentColor         string `json:"accent_color"`
	BorderColor         string `json:"border_color"`
}

// SiteAdsScriptJSON stores header/footer scripts, ad slots, and in-article ads.
type SiteAdsScriptJSON struct {
	HeaderScript       string `json:"header_script"`
	FooterScript       string `json:"footer_script"`
	AdsTop             string `json:"ads_top"`
	AdsCenter          string `json:"ads_center"`
	AdsBottom          string `json:"ads_bottom"`
	AdsLazyLoadEnabled bool   `json:"ads_lazy_load_enabled"`
	ArticleAdsJSON     string `json:"article_ads_json"`
}

// SiteImgResizeSelfJSON stores self-hosted /img proxy and stored-image limits.
type SiteImgResizeSelfJSON struct {
	Enabled        bool   `json:"enabled"`
	Width          string `json:"width"`
	Height         string `json:"height"`
	Quality        string `json:"quality"`
	StoredMaxWidth string `json:"stored_max_width"`
	StoredQuality  string `json:"stored_quality"`
	AvatarWidth    string `json:"avatar_width"`
	AvatarQuality  string `json:"avatar_quality"`
}

// SiteImgResizeCDNJSON stores Cloudflare /cdn-cgi/image/ delivery settings.
type SiteImgResizeCDNJSON struct {
	Enabled bool   `json:"enabled"`
	Width   string `json:"width"`
	Quality string `json:"quality"`
}

// SiteSocialLinksJSON stores social profile URLs.
type SiteSocialLinksJSON struct {
	FacebookURL  string `json:"facebook_url"`
	XURL         string `json:"x_url"`
	InstagramURL string `json:"instagram_url"`
	YouTubeURL   string `json:"youtube_url"`
	TikTokURL    string `json:"tiktok_url"`
	RSSURL       string `json:"rss_url"`
}

// SiteTypographyJSON stores font family and size tokens.
type SiteTypographyJSON struct {
	FontFamily      string `json:"font_family"`
	ContentFontSize string `json:"content_font_size"`
	SidebarFontSize string `json:"sidebar_font_size"`
}

// SiteSidebarConfigJSON stores sidebar list limits.
type SiteSidebarConfigJSON struct {
	Limit        int `json:"limit"`
	LimitPopular int `json:"limit_popular"`
	LimitLatest  int `json:"limit_latest"`
}

// SiteImageCaptionJSON stores image caption settings.
type SiteImageCaptionJSON struct {
	Enabled  bool   `json:"enabled"`
	Mode     string `json:"mode"`
	Template string `json:"template"`
}

// SiteDefaultMetaJSON stores default SEO meta values.
type SiteDefaultMetaJSON struct {
	Title    string `json:"title"`
	Desc     string `json:"description"`
	Image    string `json:"image"`
	Keywords string `json:"keywords"`
}

// SiteSourceDisclaimerJSON stores post source disclaimer settings.
type SiteSourceDisclaimerJSON struct {
	Mode     string `json:"mode"`
	Template string `json:"template"`
}

// SiteAlsoReadJSON stores in-body "Baca juga" settings.
type SiteAlsoReadJSON struct {
	Enabled           bool `json:"enabled"`
	MaxInserts        int  `json:"max_inserts"`
	ParagraphInterval int  `json:"paragraph_interval"`
}

// SitePostDateConfigJSON stores post date refresh / URL recycle worker settings.
type SitePostDateConfigJSON struct {
	RefreshEnabled         bool `json:"refresh_enabled"`
	RefreshDays            int  `json:"refresh_days"`
	IntervalMins           int  `json:"interval_mins"`
	UrlRecycleEnabled      bool `json:"url_recycle_enabled"`
	UrlRecycleIntervalMins int  `json:"url_recycle_interval_mins"`
}

// SiteSEOFilesJSON stores ads.txt and robots.txt content.
type SiteSEOFilesJSON struct {
	AdsTxt    string `json:"ads_txt"`
	RobotsTxt string `json:"robots_txt"`
}

func marshalSiteSettingJSON(v any) string {
	if v == nil {
		return ""
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func unmarshalSiteSettingJSON(raw string, dst any) {
	raw = strings.TrimSpace(raw)
	if raw == "" || dst == nil {
		return
	}
	_ = json.Unmarshal([]byte(raw), dst)
}

func isJSONBlob(raw string) bool {
	return IsJSONBlob(raw)
}

// IsJSONBlob reports whether raw looks like a persisted JSON object column value.
func IsJSONBlob(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "{")
}

// JSONBlobEmpty reports whether a JSON column has no stored payload yet.
func JSONBlobEmpty(raw string) bool {
	return strings.TrimSpace(raw) == ""
}

// PackSiteSettingJSON copies flat runtime fields into persisted JSON columns.
func PackSiteSettingJSON(s *SiteSetting) {
	if s == nil {
		return
	}
	s.StoredColorTheme = marshalSiteSettingJSON(SiteColorThemeJSON{
		Preset:              NormalizeColorTheme(s.ColorTheme),
		TextPrimary:         s.TextPrimary,
		TextSecondary:       s.TextSecondary,
		BackgroundPrimary:   s.BackgroundPrimary,
		BackgroundSecondary: s.BackgroundSecondary,
		PrimaryButton:       s.PrimaryButton,
		SecondaryButton:     s.SecondaryButton,
		AccentColor:         s.AccentColor,
		BorderColor:         s.BorderColor,
	})
	s.StoredAdsScript = marshalSiteSettingJSON(SiteAdsScriptJSON{
		HeaderScript:       s.HeaderScript,
		FooterScript:       s.FooterScript,
		AdsTop:             s.AdsTop,
		AdsCenter:          s.AdsCenter,
		AdsBottom:          s.AdsBottom,
		AdsLazyLoadEnabled: s.AdsLazyLoadEnabled,
		ArticleAdsJSON:     s.ArticleAdsJSON,
	})
	s.StoredImgResizeSelf = marshalSiteSettingJSON(SiteImgResizeSelfJSON{
		Enabled:        s.ImgResizeEnabled,
		Width:          s.ImgResizeWidth,
		Height:         s.ImgResizeHeight,
		Quality:        s.ImgResizeQuality,
		StoredMaxWidth: s.StoredImageMaxWidth,
		StoredQuality:  s.StoredImageQuality,
		AvatarWidth:    s.AvatarResizeWidth,
		AvatarQuality:  s.AvatarResizeQuality,
	})
	s.StoredImgResizeCDN = marshalSiteSettingJSON(SiteImgResizeCDNJSON{
		Enabled: s.ImageResizeEnabled,
		Width:   s.ImageResizeWidth,
		Quality: s.ImageResizeQuality,
	})
	s.StoredSocialLinks = marshalSiteSettingJSON(SiteSocialLinksJSON{
		FacebookURL:  s.SocialFacebookURL,
		XURL:         s.SocialXURL,
		InstagramURL: s.SocialInstagramURL,
		YouTubeURL:   s.SocialYouTubeURL,
		TikTokURL:    s.SocialTikTokURL,
		RSSURL:       s.SocialRSSURL,
	})
	s.StoredTypography = marshalSiteSettingJSON(SiteTypographyJSON{
		FontFamily:      s.FontFamily,
		ContentFontSize: s.ContentFontSize,
		SidebarFontSize: s.SidebarFontSize,
	})
	s.StoredSidebarConfig = marshalSiteSettingJSON(SiteSidebarConfigJSON{
		Limit:        s.SidebarListLimit,
		LimitPopular: s.SidebarListLimitPopular,
		LimitLatest:  s.SidebarListLimitLatest,
	})
	s.StoredImageCaption = marshalSiteSettingJSON(SiteImageCaptionJSON{
		Enabled:  s.ImageCaptionEnabled,
		Mode:     s.ImageCaptionMode,
		Template: s.ImageCaptionTemplate,
	})
	s.StoredDefaultMeta = marshalSiteSettingJSON(SiteDefaultMetaJSON{
		Title:    s.DefaultTitle,
		Desc:     s.DefaultDesc,
		Image:    s.DefaultImage,
		Keywords: s.DefaultKeywords,
	})
	s.StoredSourceDisclaimer = marshalSiteSettingJSON(SiteSourceDisclaimerJSON{
		Mode:     s.SourceDisclaimerMode,
		Template: s.SourceDisclaimerTemplate,
	})
	s.StoredAlsoRead = marshalSiteSettingJSON(SiteAlsoReadJSON{
		Enabled:           s.AlsoReadEnabled,
		MaxInserts:        s.AlsoReadMaxInserts,
		ParagraphInterval: s.AlsoReadParagraphInterval,
	})
	s.StoredPostDateConfig = marshalSiteSettingJSON(SitePostDateConfigJSON{
		RefreshEnabled:         s.PostDateRefreshEnabled,
		RefreshDays:            s.PostDateRefreshDays,
		IntervalMins:           s.PostDateIntervalMins,
		UrlRecycleEnabled:      s.PostDateUrlRecycleEnabled,
		UrlRecycleIntervalMins: s.PostDateUrlRecycleIntervalMins,
	})
	s.StoredSeoFiles = marshalSiteSettingJSON(SiteSEOFilesJSON{
		AdsTxt:    s.AdsTxt,
		RobotsTxt: s.RobotsTxt,
	})
}

// UnpackSiteSettingJSON expands persisted JSON columns into flat runtime fields.
func UnpackSiteSettingJSON(s *SiteSetting) {
	if s == nil {
		return
	}
	if strings.TrimSpace(s.StoredColorTheme) != "" {
		if isJSONBlob(s.StoredColorTheme) {
			var v SiteColorThemeJSON
			unmarshalSiteSettingJSON(s.StoredColorTheme, &v)
			s.ColorTheme = NormalizeColorTheme(v.Preset)
			s.TextPrimary = v.TextPrimary
			s.TextSecondary = v.TextSecondary
			s.BackgroundPrimary = v.BackgroundPrimary
			s.BackgroundSecondary = v.BackgroundSecondary
			s.PrimaryButton = v.PrimaryButton
			s.SecondaryButton = v.SecondaryButton
			s.AccentColor = v.AccentColor
			s.BorderColor = v.BorderColor
		} else {
			s.ColorTheme = NormalizeColorTheme(s.StoredColorTheme)
		}
	}

	var ads SiteAdsScriptJSON
	unmarshalSiteSettingJSON(s.StoredAdsScript, &ads)
	if s.StoredAdsScript != "" || ads != (SiteAdsScriptJSON{}) {
		s.HeaderScript = ads.HeaderScript
		s.FooterScript = ads.FooterScript
		s.AdsTop = ads.AdsTop
		s.AdsCenter = ads.AdsCenter
		s.AdsBottom = ads.AdsBottom
		s.AdsLazyLoadEnabled = ads.AdsLazyLoadEnabled
		s.ArticleAdsJSON = ads.ArticleAdsJSON
	}

	var self SiteImgResizeSelfJSON
	unmarshalSiteSettingJSON(s.StoredImgResizeSelf, &self)
	if s.StoredImgResizeSelf != "" || self != (SiteImgResizeSelfJSON{}) {
		s.ImgResizeEnabled = self.Enabled
		s.ImgResizeWidth = self.Width
		s.ImgResizeHeight = self.Height
		s.ImgResizeQuality = self.Quality
		s.StoredImageMaxWidth = self.StoredMaxWidth
		s.StoredImageQuality = self.StoredQuality
		s.AvatarResizeWidth = self.AvatarWidth
		s.AvatarResizeQuality = self.AvatarQuality
	}

	var cdn SiteImgResizeCDNJSON
	unmarshalSiteSettingJSON(s.StoredImgResizeCDN, &cdn)
	if s.StoredImgResizeCDN != "" || cdn != (SiteImgResizeCDNJSON{}) {
		s.ImageResizeEnabled = cdn.Enabled
		s.ImageResizeWidth = cdn.Width
		s.ImageResizeQuality = cdn.Quality
	}

	var social SiteSocialLinksJSON
	unmarshalSiteSettingJSON(s.StoredSocialLinks, &social)
	if s.StoredSocialLinks != "" || social != (SiteSocialLinksJSON{}) {
		s.SocialFacebookURL = social.FacebookURL
		s.SocialXURL = social.XURL
		s.SocialInstagramURL = social.InstagramURL
		s.SocialYouTubeURL = social.YouTubeURL
		s.SocialTikTokURL = social.TikTokURL
		s.SocialRSSURL = social.RSSURL
	}

	var typo SiteTypographyJSON
	unmarshalSiteSettingJSON(s.StoredTypography, &typo)
	if s.StoredTypography != "" || typo != (SiteTypographyJSON{}) {
		s.FontFamily = typo.FontFamily
		s.ContentFontSize = typo.ContentFontSize
		s.SidebarFontSize = typo.SidebarFontSize
	}

	var sidebar SiteSidebarConfigJSON
	unmarshalSiteSettingJSON(s.StoredSidebarConfig, &sidebar)
	if s.StoredSidebarConfig != "" || sidebar != (SiteSidebarConfigJSON{}) {
		s.SidebarListLimit = sidebar.Limit
		s.SidebarListLimitPopular = sidebar.LimitPopular
		s.SidebarListLimitLatest = sidebar.LimitLatest
	}

	var caption SiteImageCaptionJSON
	unmarshalSiteSettingJSON(s.StoredImageCaption, &caption)
	if s.StoredImageCaption != "" || caption != (SiteImageCaptionJSON{}) {
		s.ImageCaptionEnabled = caption.Enabled
		s.ImageCaptionMode = caption.Mode
		s.ImageCaptionTemplate = caption.Template
	}

	var meta SiteDefaultMetaJSON
	unmarshalSiteSettingJSON(s.StoredDefaultMeta, &meta)
	if s.StoredDefaultMeta != "" || meta != (SiteDefaultMetaJSON{}) {
		s.DefaultTitle = meta.Title
		s.DefaultDesc = meta.Desc
		s.DefaultImage = meta.Image
		s.DefaultKeywords = meta.Keywords
	}

	var disclaimer SiteSourceDisclaimerJSON
	unmarshalSiteSettingJSON(s.StoredSourceDisclaimer, &disclaimer)
	if s.StoredSourceDisclaimer != "" || disclaimer != (SiteSourceDisclaimerJSON{}) {
		s.SourceDisclaimerMode = disclaimer.Mode
		s.SourceDisclaimerTemplate = disclaimer.Template
	}

	var alsoRead SiteAlsoReadJSON
	unmarshalSiteSettingJSON(s.StoredAlsoRead, &alsoRead)
	if s.StoredAlsoRead != "" || alsoRead != (SiteAlsoReadJSON{}) {
		s.AlsoReadEnabled = alsoRead.Enabled
		s.AlsoReadMaxInserts = alsoRead.MaxInserts
		s.AlsoReadParagraphInterval = EffectiveAlsoReadParagraphInterval(alsoRead.ParagraphInterval)
	}

	var postDate SitePostDateConfigJSON
	unmarshalSiteSettingJSON(s.StoredPostDateConfig, &postDate)
	if s.StoredPostDateConfig != "" || postDate != (SitePostDateConfigJSON{}) {
		s.PostDateRefreshEnabled = postDate.RefreshEnabled
		s.PostDateRefreshDays = postDate.RefreshDays
		s.PostDateIntervalMins = postDate.IntervalMins
		s.PostDateUrlRecycleEnabled = postDate.UrlRecycleEnabled
		s.PostDateUrlRecycleIntervalMins = postDate.UrlRecycleIntervalMins
	}

	var seo SiteSEOFilesJSON
	unmarshalSiteSettingJSON(s.StoredSeoFiles, &seo)
	if s.StoredSeoFiles != "" || seo != (SiteSEOFilesJSON{}) {
		s.AdsTxt = seo.AdsTxt
		s.RobotsTxt = seo.RobotsTxt
	}
}

// JSONColumnsComplete reports whether all persisted JSON blob columns are populated.
func (s *SiteSetting) JSONColumnsComplete() bool {
	if s == nil {
		return false
	}
	return IsJSONBlob(s.StoredColorTheme) &&
		!JSONBlobEmpty(s.StoredAdsScript) &&
		!JSONBlobEmpty(s.StoredImgResizeSelf) &&
		!JSONBlobEmpty(s.StoredImgResizeCDN) &&
		!JSONBlobEmpty(s.StoredSocialLinks) &&
		!JSONBlobEmpty(s.StoredTypography) &&
		!JSONBlobEmpty(s.StoredSidebarConfig) &&
		!JSONBlobEmpty(s.StoredImageCaption) &&
		!JSONBlobEmpty(s.StoredDefaultMeta) &&
		!JSONBlobEmpty(s.StoredSourceDisclaimer) &&
		!JSONBlobEmpty(s.StoredAlsoRead) &&
		!JSONBlobEmpty(s.StoredPostDateConfig) &&
		!JSONBlobEmpty(s.StoredSeoFiles)
}

// SiteSettingJSONColumnUpdates returns a map of JSON column values after PackSiteSettingJSON.
func SiteSettingJSONColumnUpdates(s *SiteSetting) map[string]interface{} {
	PackSiteSettingJSON(s)
	if s == nil {
		return nil
	}
	return map[string]interface{}{
		"color_theme":       s.StoredColorTheme,
		"ads_script":        s.StoredAdsScript,
		"img_resize_self":   s.StoredImgResizeSelf,
		"img_resize_cdn":    s.StoredImgResizeCDN,
		"social_links":      s.StoredSocialLinks,
		"typography":        s.StoredTypography,
		"sidebar_config":    s.StoredSidebarConfig,
		"image_caption":     s.StoredImageCaption,
		"default_meta":      s.StoredDefaultMeta,
		"source_disclaimer": s.StoredSourceDisclaimer,
		"also_read":         s.StoredAlsoRead,
		"post_date_config":  s.StoredPostDateConfig,
		"seo_files":         s.StoredSeoFiles,
	}
}

// BeforeCreate packs runtime fields into JSON columns before insert (Create only; UpdateColumns skips hooks).
func (s *SiteSetting) BeforeCreate(_ *gorm.DB) error {
	PackSiteSettingJSON(s)
	return nil
}

// AfterFind unpacks JSON columns after loading from the database.
func (s *SiteSetting) AfterFind(_ *gorm.DB) error {
	UnpackSiteSettingJSON(s)
	return nil
}

func (s *SiteSetting) ColorThemeRaw() string       { return s.StoredColorTheme }
func (s *SiteSetting) AdsScriptRaw() string        { return s.StoredAdsScript }
func (s *SiteSetting) ImgResizeSelfRaw() string    { return s.StoredImgResizeSelf }
func (s *SiteSetting) ImgResizeCDNRaw() string     { return s.StoredImgResizeCDN }
func (s *SiteSetting) SocialLinksRaw() string      { return s.StoredSocialLinks }
func (s *SiteSetting) TypographyRaw() string       { return s.StoredTypography }
func (s *SiteSetting) SidebarConfigRaw() string    { return s.StoredSidebarConfig }
func (s *SiteSetting) ImageCaptionRaw() string     { return s.StoredImageCaption }
func (s *SiteSetting) DefaultMetaRaw() string      { return s.StoredDefaultMeta }
func (s *SiteSetting) SourceDisclaimerRaw() string { return s.StoredSourceDisclaimer }
func (s *SiteSetting) AlsoReadRaw() string         { return s.StoredAlsoRead }
func (s *SiteSetting) PostDateConfigRaw() string   { return s.StoredPostDateConfig }
func (s *SiteSetting) SeoFilesRaw() string         { return s.StoredSeoFiles }
