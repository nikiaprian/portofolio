package seeds

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedSiteSetting(db *gorm.DB) error {
	siteSetting := entity.SiteSetting{
		SiteName:                 "Portfolio",
		Tagline:                  "Portfolio online dengan subdomain sendiri.",
		LogoURL:                  "",
		FaviconURL:               "",
		TextPrimary:              "#2f3640",
		TextSecondary:            "#6c757d",
		BackgroundPrimary:        "#ffffff",
		BackgroundSecondary:      "#f8f9fa",
		PrimaryButton:            "#0d6efd",
		SecondaryButton:          "#6c757d",
		AccentColor:              "#c62828",
		BorderColor:              "#ced4da",
		NotFoundRedirectMode:     entity.NotFoundRedirectHome,
		DefaultTitle:             "Portfolio",
		DefaultDesc:              "Portfolio online. Beli sekali, dapat subdomain, lalu isi foto, bio, keahlian, proyek, dan kontak.",
		DefaultImage:             "",
		DefaultKeywords:          "portfolio, subdomain, karya, kontak",
		HeaderScript:             "",
		FooterScript:             "",
		AdsTop:                   "",
		AdsCenter:                "",
		AdsBottom:                "",
		HomepageLayout:           entity.HomeLayoutDefault,
		ColorTheme:               entity.ColorThemeDefault,
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
