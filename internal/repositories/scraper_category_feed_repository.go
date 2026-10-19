package repositories

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// ScraperCategoryFeedRepository persists per-category scraper feed URLs.
type ScraperCategoryFeedRepository interface {
	List() ([]entity.ScraperCategoryFeed, error)
	ReplaceAll(rows []entity.ScraperCategoryFeed) error
	// DedupeByCanonicalSlug deletes extra rows when LOWER(TRIM(category_slug)) collides (keeps lowest id).
	DedupeByCanonicalSlug() error
}

type scraperCategoryFeedRepository struct {
	db *gorm.DB
}

// NewScraperCategoryFeedRepository constructs ScraperCategoryFeedRepository.
func NewScraperCategoryFeedRepository(db *gorm.DB) ScraperCategoryFeedRepository {
	return &scraperCategoryFeedRepository{db: db}
}

func (r *scraperCategoryFeedRepository) List() ([]entity.ScraperCategoryFeed, error) {
	var out []entity.ScraperCategoryFeed
	err := r.db.Order("id asc").Find(&out).Error
	return out, err
}

func (r *scraperCategoryFeedRepository) DedupeByCanonicalSlug() error {
	var rows []entity.ScraperCategoryFeed
	if err := r.db.Order("id asc").Find(&rows).Error; err != nil {
		return err
	}
	seen := make(map[string]bool, len(rows))
	var dupIDs []uint
	for _, row := range rows {
		k := entity.CanonicalCategorySlug(row.CategorySlug)
		if k == "" {
			continue
		}
		if seen[k] {
			dupIDs = append(dupIDs, row.ID)
			continue
		}
		seen[k] = true
	}
	if len(dupIDs) == 0 {
		return nil
	}
	return r.db.Delete(&entity.ScraperCategoryFeed{}, dupIDs).Error
}

func (r *scraperCategoryFeedRepository) ReplaceAll(rows []entity.ScraperCategoryFeed) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&entity.ScraperCategoryFeed{}).Error; err != nil {
			return err
		}
		for i := range rows {
			// Use a map so MySQL receives explicit zero-values too.
			payload := map[string]interface{}{
				"category_slug":     rows[i].CategorySlug,
				"feed_urls":         rows[i].FeedURLs,
				"enabled":           rows[i].Enabled,
				"include_source":    rows[i].IncludeSource,
				"article_status":    rows[i].ArticleStatus,
				"image_mode":        rows[i].ImageMode,
				"web_search_source": rows[i].WebSearchSource,
				"image_to_content":  rows[i].ImageToContent,
				"auto_tag":          rows[i].AutoTag,
				"author_email":      rows[i].AuthorEmail,
			}
			if err := tx.Model(&entity.ScraperCategoryFeed{}).Create(payload).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
