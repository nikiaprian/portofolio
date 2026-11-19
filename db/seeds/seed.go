package seeds

import (
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
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
		if err := renameLegacyTirtayasaSite(db); err != nil {
			return err
		}
		if err := EnsureAISettingDefaults(db); err != nil {
			return err
		}
		if err := seedPages(db); err != nil {
			return err
		}
		if err := seedSiteWebImageProviders(db); err != nil {
			return err
		}
		// Categories were skipped on the first install. FirstOrCreate fills only missing slugs.
		if _, err := seedCategories(db); err != nil {
			return err
		}
		return seedScraperCategoryFeeds(db)
	}

	if _, err := seedUsers(db); err != nil {
		return err
	}
	if _, err := seedCategories(db); err != nil {
		return err
	}
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

	return nil
}

// renameLegacyTirtayasaSite rewrites a fresh news install that still carries the old site name.
func renameLegacyTirtayasaSite(db *gorm.DB) error {
	var row entity.SiteSetting
	if err := db.Order("id asc").First(&row).Error; err != nil {
		return nil
	}
	if strings.TrimSpace(row.SiteName) != "Tirtayasa" && strings.TrimSpace(row.DefaultTitle) != "Tirtayasa" {
		return nil
	}
	row.SiteName = "Seodigital"
	row.Tagline = "Berita dan informasi terkini."
	row.DefaultTitle = "Seodigital"
	row.DefaultDesc = "Berita dan informasi terkini dari redaksi Seodigital."
	row.DefaultKeywords = "berita, informasi, Seodigital"
	return db.Model(&row).
		Select("SiteName", "Tagline", "DefaultTitle", "DefaultDesc", "DefaultKeywords").
		Updates(&row).Error
}
