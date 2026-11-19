package seeds

import (
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func seedSiteSetting(db *gorm.DB) error {
	siteSetting := entity.SiteSetting{
		SiteName:                 "Seodigital",
		Tagline:                  "Berita dan informasi terkini.",
		LogoURL:                  "",
		FaviconURL:               "",
		TextPrimary:              "#1e293b",
		TextSecondary:            "#64748b",
		BackgroundPrimary:        "#ffffff",
		BackgroundSecondary:      "#ffffff",
		PrimaryButton:            "#166534",
		SecondaryButton:          "#64748b",
		AccentColor:              "#166534",
		BorderColor:              "#e5e7eb",
		NotFoundRedirectMode:     entity.NotFoundRedirectHome,
		DefaultTitle:             "Seodigital",
		DefaultDesc:              "Berita dan informasi terkini dari redaksi Seodigital.",
		DefaultImage:             "",
		DefaultKeywords:          "berita, informasi, Seodigital",
		HeaderScript:             "",
		FooterScript:             "",
		AdsTop:                   "",
		AdsCenter:                "",
		AdsBottom:                "",
		HomepageLayout:           entity.HomeLayoutTirtayasa,
		ColorTheme:               entity.ColorThemeCustom,
		SocialFacebookURL:        "",
		SocialXURL:               "",
		SocialInstagramURL:       "",
		SocialYouTubeURL:         "",
		SocialRSSURL:             "",
		LoginURL:                 "",
		ImageResizeEnabled:       true,
		PostDateRefreshEnabled:   false,
		PostDateRefreshDays:      30,
		PostDateIntervalMins:     30,
		SourceDisclaimerMode:     entity.SourceDisclaimerDefault,
		SourceDisclaimerTemplate: entity.DefaultSourceDisclaimerTemplate,
		ImageCaptionMode:         entity.ImageCaptionModeDefault,
		ImageCaptionTemplate:     entity.DefaultImageCaptionTemplate,
	}
	entity.PackSiteSettingJSON(&siteSetting)
	return db.Create(&siteSetting).Error
}
