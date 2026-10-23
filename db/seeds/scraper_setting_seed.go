package seeds

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func seedScraperSetting(db *gorm.DB) error {
	// Prevent duplicate "singleton" rows when seeds are re-run.
	var n int64
	if err := db.Model(&entity.ScraperSetting{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	scraperSetting := entity.ScraperSetting{
		Enabled:      false,
		IntervalMins: 5,
		AuthorEmail:  "admin@tirtayasa.id",
	}
	return db.Create(&scraperSetting).Error
}

func seedScraperCategoryFeeds(db *gorm.DB) error {
	var cats []entity.Category
	if err := db.Order("id asc").Find(&cats).Error; err != nil {
		return err
	}
	for i, c := range cats {
		catSlug := strings.TrimSpace(c.Slug)
		canon := entity.CanonicalCategorySlug(catSlug)
		var existing entity.ScraperCategoryFeed
		err := db.Where("LOWER(TRIM(category_slug)) = ?", canon).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		feed := ""
		if i == 0 {
			feed = "https://www.cnnindonesia.com/nasional/rss"
		}
		row := entity.ScraperCategoryFeed{
			CategorySlug:  catSlug,
			FeedURLs:      feed,
			Enabled:       false,
			IncludeSource: true,
			ArticleStatus: "published",
			ImageMode:     "scrape",
			AutoTag:       true,
			AuthorEmail:   "admin@tirtayasa.id",
		}
		if err := db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
