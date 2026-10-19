package seeds

import (
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func normalizeSeedLocale(locale string) string {
	if strings.EqualFold(strings.TrimSpace(locale), "en") {
		return "en"
	}
	return "id"
}

// function to run the seed
func Run(db *gorm.DB) error {
	// if site setting already exists, assume seeds have been run
	var settingCount int64
	if err := db.Model(&entity.SiteSetting{}).Count(&settingCount).Error; err != nil {
		return err
	}
	if settingCount > 0 {
		// Full seed already ran before; still backfill newer default rows/settings.
		if err := EnsureAISettingDefaults(db); err != nil {
			return err
		}
		if err := seedSiteLayout(db); err != nil {
			return err
		}
		if err := ensurePortofoliokuSiteDefaults(db); err != nil {
			return err
		}
		return seedSiteWebImageProviders(db)
	}

	if _, err := seedUsers(db); err != nil {
		return err
	}
	// if _, err := seedCategories(db); err != nil {
	// 	return err
	// }
	if err := seedSiteSetting(db); err != nil {
		return err
	}
	if err := seedSiteConnection(db); err != nil {
		return err
	}
	if err := seedSiteCloudStorage(db); err != nil {
		return err
	}
	if err := seedSiteWebImageProviders(db); err != nil {
		return err
	}
	if err := seedScraperSetting(db); err != nil {
		return err
	}
	if err := seedScraperCategoryFeeds(db); err != nil {
		return err
	}
	if err := EnsureAISettingDefaults(db); err != nil {
		return err
	}
	if err := seedPages(db); err != nil {
		return err
	}
	if err := seedSiteLayout(db); err != nil {
		return err
	}

	return nil
}
