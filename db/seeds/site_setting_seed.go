package seeds

import (
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func seedSiteSetting(db *gorm.DB) error {
	siteSetting := entity.SiteSetting{
		SiteName:                 "Tirtayasa",
		Tagline:                  "Catatan dan artikel dari redaksi Tirtayasa.",
		LogoURL:                  "",
		FaviconURL:               "",
		TextPrimary:              "#1c1915",
		TextSecondary:            "#5c564c",
		BackgroundPrimary:        "#ffffff",
		BackgroundSecondary:      "#ffffff",
		PrimaryButton:            "#0f3d3e",
		SecondaryButton:          "#8a8175",
		AccentColor:              "#0f3d3e",
		BorderColor:              "#e4d9c8",
		NotFoundRedirectMode:     entity.NotFoundRedirectHome,
		DefaultTitle:             "Tirtayasa",
		DefaultDesc:              "Tirtayasa menerbitkan artikel orisinal yang mudah dibaca di tirtayasa.id.",
		DefaultImage:             "",
		DefaultKeywords:          "tirtayasa, artikel, bacaan",
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
