package entity

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"
)

// JSON columns below match the shared bebii_blog schema. Runtime fields stay on SiteSetting
// so templates and admin forms keep reading the same names.

type siteColorThemeJSON struct {
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

type siteAdsScriptJSON struct {
	HeaderScript       string `json:"header_script"`
	FooterScript       string `json:"footer_script"`
	AdsTop             string `json:"ads_top"`
	AdsCenter          string `json:"ads_center"`
	AdsBottom          string `json:"ads_bottom"`
	AdsLazyLoadEnabled bool   `json:"ads_lazy_load_enabled"`
	ArticleAdsJSON     string `json:"article_ads_json"`
}

type siteImgResizeSelfJSON struct {
	Enabled        bool   `json:"enabled"`
	Width          string `json:"width"`
	Height         string `json:"height"`
	Quality        string `json:"quality"`
	StoredMaxWidth string `json:"stored_max_width"`
	StoredQuality  string `json:"stored_quality"`
	AvatarWidth    string `json:"avatar_width"`
	AvatarQuality  string `json:"avatar_quality"`
}

type siteImgResizeCDNJSON struct {
	Enabled bool   `json:"enabled"`
	Width   string `json:"width"`
	Quality string `json:"quality"`
}

type siteSocialLinksJSON struct {
	FacebookURL  string `json:"facebook_url"`
	XURL         string `json:"x_url"`
	InstagramURL string `json:"instagram_url"`
	YouTubeURL   string `json:"youtube_url"`
	TikTokURL    string `json:"tiktok_url"`
	RSSURL       string `json:"rss_url"`
}

type siteTypographyJSON struct {
	FontFamily      string `json:"font_family"`
	ContentFontSize string `json:"content_font_size"`
	SidebarFontSize string `json:"sidebar_font_size"`
}

type siteSidebarConfigJSON struct {
	Limit        int `json:"limit"`
	LimitPopular int `json:"limit_popular"`
	LimitLatest  int `json:"limit_latest"`
}

type siteImageCaptionJSON struct {
	Enabled  bool   `json:"enabled"`
	Mode     string `json:"mode"`
	Template string `json:"template"`
}

type siteDefaultMetaJSON struct {
	Title    string `json:"title"`
	Desc     string `json:"description"`
	Image    string `json:"image"`
	Keywords string `json:"keywords"`
}

type siteSourceDisclaimerJSON struct {
	Mode     string `json:"mode"`
	Template string `json:"template"`
}

type siteAlsoReadJSON struct {
	Enabled           bool `json:"enabled"`
	MaxInserts        int  `json:"max_inserts"`
	ParagraphInterval int  `json:"paragraph_interval"`
}

type sitePostDateConfigJSON struct {
	RefreshEnabled         bool `json:"refresh_enabled"`
	RefreshDays            int  `json:"refresh_days"`
	IntervalMins           int  `json:"interval_mins"`
	UrlRecycleEnabled      bool `json:"url_recycle_enabled"`
	UrlRecycleIntervalMins int  `json:"url_recycle_interval_mins"`
}

type siteSEOFilesJSON struct {
	AdsTxt    string `json:"ads_txt"`
	RobotsTxt string `json:"robots_txt"`
}

func marshalSiteSettingJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func unmarshalSiteSettingJSON(raw string, dst any) {
	raw = strings.TrimSpace(raw)
	if raw == "" || dst == nil || !strings.HasPrefix(raw, "{") {
		return
	}
	_ = json.Unmarshal([]byte(raw), dst)
}

// PackSiteSettingJSON copies runtime fields into the shared JSON columns.
func PackSiteSettingJSON(s *SiteSetting) {
	if s == nil {
		return
	}
	s.StoredColorTheme = marshalSiteSettingJSON(siteColorThemeJSON{
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
	s.StoredAdsScript = marshalSiteSettingJSON(siteAdsScriptJSON{
		HeaderScript:       s.HeaderScript,
		FooterScript:       s.FooterScript,
		AdsTop:             s.AdsTop,
		AdsCenter:          s.AdsCenter,
		AdsBottom:          s.AdsBottom,
		AdsLazyLoadEnabled: s.AdsLazyLoadEnabled,
		ArticleAdsJSON:     s.ArticleAdsJSON,
	})
	s.StoredImgResizeSelf = marshalSiteSettingJSON(siteImgResizeSelfJSON{
		Enabled:        s.ImgResizeEnabled,
		Width:          s.ImgResizeWidth,
		Height:         s.ImgResizeHeight,
		Quality:        s.ImgResizeQuality,
		StoredMaxWidth: s.StoredImageMaxWidth,
		StoredQuality:  s.StoredImageQuality,
		AvatarWidth:    s.AvatarResizeWidth,
		AvatarQuality:  s.AvatarResizeQuality,
	})
	s.StoredImgResizeCDN = marshalSiteSettingJSON(siteImgResizeCDNJSON{
		Enabled: s.ImageResizeEnabled,
		Width:   s.ImageResizeWidth,
		Quality: s.ImageResizeQuality,
	})
	s.StoredSocialLinks = marshalSiteSettingJSON(siteSocialLinksJSON{
		FacebookURL:  s.SocialFacebookURL,
		XURL:         s.SocialXURL,
		InstagramURL: s.SocialInstagramURL,
		YouTubeURL:   s.SocialYouTubeURL,
		TikTokURL:    s.SocialTikTokURL,
		RSSURL:       s.SocialRSSURL,
	})
	s.StoredTypography = marshalSiteSettingJSON(siteTypographyJSON{
		FontFamily:      s.FontFamily,
		ContentFontSize: s.ContentFontSize,
		SidebarFontSize: s.SidebarFontSize,
	})
	s.StoredSidebarConfig = marshalSiteSettingJSON(siteSidebarConfigJSON{
		Limit:        s.SidebarListLimit,
		LimitPopular: s.SidebarListLimitPopular,
		LimitLatest:  s.SidebarListLimitLatest,
	})
	s.StoredImageCaption = marshalSiteSettingJSON(siteImageCaptionJSON{
		Enabled:  s.ImageCaptionEnabled,
		Mode:     s.ImageCaptionMode,
		Template: s.ImageCaptionTemplate,
	})
	s.StoredDefaultMeta = marshalSiteSettingJSON(siteDefaultMetaJSON{
		Title:    s.DefaultTitle,
		Desc:     s.DefaultDesc,
		Image:    s.DefaultImage,
		Keywords: s.DefaultKeywords,
	})
	s.StoredSourceDisclaimer = marshalSiteSettingJSON(siteSourceDisclaimerJSON{
		Mode:     s.SourceDisclaimerMode,
		Template: s.SourceDisclaimerTemplate,
	})
	s.StoredAlsoRead = marshalSiteSettingJSON(siteAlsoReadJSON{
		Enabled:           s.AlsoReadEnabled,
		MaxInserts:        s.AlsoReadMaxInserts,
		ParagraphInterval: s.AlsoReadParagraphInterval,
	})
	s.StoredPostDateConfig = marshalSiteSettingJSON(sitePostDateConfigJSON{
		RefreshEnabled:         s.PostDateRefreshEnabled,
		RefreshDays:            s.PostDateRefreshDays,
		IntervalMins:           s.PostDateIntervalMins,
		UrlRecycleEnabled:      s.PostDateUrlRecycleEnabled,
		UrlRecycleIntervalMins: s.PostDateUrlRecycleIntervalMins,
	})
	s.StoredSeoFiles = marshalSiteSettingJSON(siteSEOFilesJSON{
		AdsTxt:    s.AdsTxt,
		RobotsTxt: s.RobotsTxt,
	})
}

// UnpackSiteSettingJSON copies shared JSON columns into the runtime fields.
func UnpackSiteSettingJSON(s *SiteSetting) {
	if s == nil {
		return
	}
	if strings.TrimSpace(s.StoredColorTheme) != "" {
		if strings.HasPrefix(strings.TrimSpace(s.StoredColorTheme), "{") {
			var v siteColorThemeJSON
			unmarshalSiteSettingJSON(s.StoredColorTheme, &v)
			s.ColorTheme = v.Preset
			s.TextPrimary = v.TextPrimary
			s.TextSecondary = v.TextSecondary
			s.BackgroundPrimary = v.BackgroundPrimary
			s.BackgroundSecondary = v.BackgroundSecondary
			s.PrimaryButton = v.PrimaryButton
			s.SecondaryButton = v.SecondaryButton
			s.AccentColor = v.AccentColor
			s.BorderColor = v.BorderColor
		} else {
			s.ColorTheme = s.StoredColorTheme
		}
	}

	if strings.TrimSpace(s.StoredAdsScript) != "" {
		var ads siteAdsScriptJSON
		unmarshalSiteSettingJSON(s.StoredAdsScript, &ads)
		s.HeaderScript = ads.HeaderScript
		s.FooterScript = ads.FooterScript
		s.AdsTop = ads.AdsTop
		s.AdsCenter = ads.AdsCenter
		s.AdsBottom = ads.AdsBottom
		s.AdsLazyLoadEnabled = ads.AdsLazyLoadEnabled
		s.ArticleAdsJSON = ads.ArticleAdsJSON
	}
	if strings.TrimSpace(s.StoredImgResizeSelf) != "" {
		var self siteImgResizeSelfJSON
		unmarshalSiteSettingJSON(s.StoredImgResizeSelf, &self)
		s.ImgResizeEnabled = self.Enabled
		s.ImgResizeWidth = self.Width
		s.ImgResizeHeight = self.Height
		s.ImgResizeQuality = self.Quality
		s.StoredImageMaxWidth = self.StoredMaxWidth
		s.StoredImageQuality = self.StoredQuality
		s.AvatarResizeWidth = self.AvatarWidth
		s.AvatarResizeQuality = self.AvatarQuality
	}
	if strings.TrimSpace(s.StoredImgResizeCDN) != "" {
		var cdn siteImgResizeCDNJSON
		unmarshalSiteSettingJSON(s.StoredImgResizeCDN, &cdn)
		s.ImageResizeEnabled = cdn.Enabled
		s.ImageResizeWidth = cdn.Width
		s.ImageResizeQuality = cdn.Quality
	}
	if strings.TrimSpace(s.StoredSocialLinks) != "" {
		var social siteSocialLinksJSON
		unmarshalSiteSettingJSON(s.StoredSocialLinks, &social)
		s.SocialFacebookURL = social.FacebookURL
		s.SocialXURL = social.XURL
		s.SocialInstagramURL = social.InstagramURL
		s.SocialYouTubeURL = social.YouTubeURL
		s.SocialTikTokURL = social.TikTokURL
		s.SocialRSSURL = social.RSSURL
	}
	if strings.TrimSpace(s.StoredTypography) != "" {
		var typo siteTypographyJSON
		unmarshalSiteSettingJSON(s.StoredTypography, &typo)
		s.FontFamily = typo.FontFamily
		s.ContentFontSize = typo.ContentFontSize
		s.SidebarFontSize = typo.SidebarFontSize
	}
	if strings.TrimSpace(s.StoredSidebarConfig) != "" {
		var sidebar siteSidebarConfigJSON
		unmarshalSiteSettingJSON(s.StoredSidebarConfig, &sidebar)
		s.SidebarListLimit = sidebar.Limit
		s.SidebarListLimitPopular = sidebar.LimitPopular
		s.SidebarListLimitLatest = sidebar.LimitLatest
	}
	if strings.TrimSpace(s.StoredImageCaption) != "" {
		var caption siteImageCaptionJSON
		unmarshalSiteSettingJSON(s.StoredImageCaption, &caption)
		s.ImageCaptionEnabled = caption.Enabled
		s.ImageCaptionMode = caption.Mode
		s.ImageCaptionTemplate = caption.Template
	}
	if strings.TrimSpace(s.StoredDefaultMeta) != "" {
		var meta siteDefaultMetaJSON
		unmarshalSiteSettingJSON(s.StoredDefaultMeta, &meta)
		s.DefaultTitle = meta.Title
		s.DefaultDesc = meta.Desc
		s.DefaultImage = meta.Image
		s.DefaultKeywords = meta.Keywords
	}
	if strings.TrimSpace(s.StoredSourceDisclaimer) != "" {
		var disclaimer siteSourceDisclaimerJSON
		unmarshalSiteSettingJSON(s.StoredSourceDisclaimer, &disclaimer)
		s.SourceDisclaimerMode = disclaimer.Mode
		s.SourceDisclaimerTemplate = disclaimer.Template
		if SourceDisclaimerAdmitsAutomaticRewrite(s.SourceDisclaimerTemplate) {
			s.SourceDisclaimerMode = SourceDisclaimerDefault
			s.SourceDisclaimerTemplate = DefaultSourceDisclaimerTemplate
		}
	}
	if strings.TrimSpace(s.StoredAlsoRead) != "" {
		var alsoRead siteAlsoReadJSON
		unmarshalSiteSettingJSON(s.StoredAlsoRead, &alsoRead)
		s.AlsoReadEnabled = alsoRead.Enabled
		s.AlsoReadMaxInserts = alsoRead.MaxInserts
		s.AlsoReadParagraphInterval = alsoRead.ParagraphInterval
	}
	if strings.TrimSpace(s.StoredPostDateConfig) != "" {
		var postDate sitePostDateConfigJSON
		unmarshalSiteSettingJSON(s.StoredPostDateConfig, &postDate)
		s.PostDateRefreshEnabled = postDate.RefreshEnabled
		s.PostDateRefreshDays = postDate.RefreshDays
		s.PostDateIntervalMins = postDate.IntervalMins
		s.PostDateUrlRecycleEnabled = postDate.UrlRecycleEnabled
		s.PostDateUrlRecycleIntervalMins = postDate.UrlRecycleIntervalMins
	}
	if strings.TrimSpace(s.StoredSeoFiles) != "" {
		var seo siteSEOFilesJSON
		unmarshalSiteSettingJSON(s.StoredSeoFiles, &seo)
		s.AdsTxt = seo.AdsTxt
		s.RobotsTxt = seo.RobotsTxt
	}
}

// BeforeSave keeps JSON columns aligned when GORM Save/Create runs.
func (s *SiteSetting) BeforeSave(*gorm.DB) error {
	PackSiteSettingJSON(s)
	return nil
}

// AfterFind hydrates runtime fields from the shared JSON columns.
func (s *SiteSetting) AfterFind(*gorm.DB) error {
	UnpackSiteSettingJSON(s)
	return nil
}
