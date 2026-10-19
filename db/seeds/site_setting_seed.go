package seeds

import (
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedSiteSetting(db *gorm.DB) error {
	siteSetting := entity.DefaultSiteSetting()
	entity.PackSiteSettingJSON(&siteSetting)
	return db.Create(&siteSetting).Error
}

// ensurePortofoliokuSiteDefaults replaces the first-install placeholder
// ("Portfolio") with the current brand and SEO defaults. A renamed site is left as saved.
func ensurePortofoliokuSiteDefaults(db *gorm.DB) error {
	var row entity.SiteSetting
	if err := db.Order("id asc").First(&row).Error; err != nil {
		return err
	}
	entity.UnpackSiteSettingJSON(&row)
	if strings.TrimSpace(row.SiteName) != "Portfolio" {
		return nil
	}
	next := entity.DefaultSiteSetting()
	next.ID = row.ID
	next.CreatedAt = row.CreatedAt
	if strings.TrimSpace(row.HeaderScript) != "" {
		next.HeaderScript = row.HeaderScript
	}
	if strings.TrimSpace(row.FooterScript) != "" {
		next.FooterScript = row.FooterScript
	}
	if strings.TrimSpace(row.AdsTop) != "" {
		next.AdsTop = row.AdsTop
	}
	if strings.TrimSpace(row.AdsCenter) != "" {
		next.AdsCenter = row.AdsCenter
	}
	if strings.TrimSpace(row.AdsBottom) != "" {
		next.AdsBottom = row.AdsBottom
	}
	if strings.TrimSpace(row.AdsTxt) != "" {
		next.AdsTxt = row.AdsTxt
	}
	if strings.TrimSpace(row.LoginURL) != "" {
		next.LoginURL = row.LoginURL
	}
	entity.PackSiteSettingJSON(&next)
	return db.Save(&next).Error
}
