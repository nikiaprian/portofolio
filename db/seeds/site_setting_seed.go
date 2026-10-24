package seeds

import (
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func seedSiteSetting(db *gorm.DB) error {
	siteSetting := entity.SiteSetting{
		SiteName:                 "Tirtayasa",
		Tagline:                  "Panduan layanan warga lewat jalur resmi.",
		LogoURL:                  "",
		FaviconURL:               "",
		TextPrimary:              "#1e293b",
		TextSecondary:            "#64748b",
		BackgroundPrimary:        "#ffffff",
		BackgroundSecondary:      "#ffffff",
		PrimaryButton:            "#1d4ed8",
		SecondaryButton:          "#64748b",
		AccentColor:              "#1d4ed8",
		BorderColor:              "#e5e7eb",
		NotFoundRedirectMode:     entity.NotFoundRedirectHome,
		DefaultTitle:             "Tirtayasa",
		DefaultDesc:              "Cara mengurus bansos, BPJS, SKCK, NISN, dan urusan kerja tanpa calo, dengan tautan ke situs pemerintah.",
		DefaultImage:             "",
		DefaultKeywords:          "bansos, BPJS, SKCK, NISN, perjanjian kerja, layanan warga",
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
