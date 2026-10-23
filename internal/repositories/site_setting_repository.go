package repositories

import (
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// SiteSettingRepository persists the singleton site_settings row.
type SiteSettingRepository interface {
	Get() (*entity.SiteSetting, error)
	Create(s *entity.SiteSetting) error
	Save(s *entity.SiteSetting) error
}

type siteSettingRepository struct {
	db *gorm.DB
}

// NewSiteSettingRepository constructs a SiteSettingRepository.
func NewSiteSettingRepository(db *gorm.DB) SiteSettingRepository {
	return &siteSettingRepository{db: db}
}

func (r *siteSettingRepository) Get() (*entity.SiteSetting, error) {
	s, err := GetSiteSettingCached(r.db)
	if err != nil {
		return nil, err
	}
	copy := s
	return &copy, nil
}

func (r *siteSettingRepository) Create(s *entity.SiteSetting) error {
	entity.PackSiteSettingJSON(s)
	if err := r.db.Create(s).Error; err != nil {
		return err
	}
	InvalidateSiteSettingCache()
	return nil
}

func (r *siteSettingRepository) Save(s *entity.SiteSetting) error {
	if s == nil {
		return nil
	}
	entity.PackSiteSettingJSON(s)
	var err error
	if s.ID == 0 {
		err = r.db.Create(s).Error
	} else {
		// Select("*") is required: UpdateColumns/Updates with a struct skip zero-value booleans (false).
		err = r.db.Model(s).Where("id = ?", s.ID).Select("*").UpdateColumns(s).Error
	}
	if err != nil {
		return err
	}
	InvalidateSiteSettingCache()
	return nil
}

// ArticleSlugModeFromDB returns site_settings.article_slug_mode for the first row (default: direct).
func ArticleSlugModeFromDB(db *gorm.DB) string {
	return entity.NormalizeArticleSlugMode(SiteSettingOrDefault(db).ArticleSlugMode)
}

// AlsoReadSettingsFromDB returns also_read_enabled and clamped also_read_max_inserts.
// Defaults: enabled true, max inserts 6 when the row is missing or on read error.
func AlsoReadSettingsFromDB(db *gorm.DB) (enabled bool, maxInserts int) {
	s := SiteSettingOrDefault(db)
	return s.AlsoReadEnabled, entity.EffectiveAlsoReadMaxInserts(s.AlsoReadMaxInserts)
}

// AlsoReadEnabledFromDB returns site_settings.also_read_enabled for the first row.
// Returns true when the row is missing or on read error (preserve legacy behavior).
func AlsoReadEnabledFromDB(db *gorm.DB) bool {
	enabled, _ := AlsoReadSettingsFromDB(db)
	return enabled
}

// AutoHrefEnabledFromDB returns site_settings.auto_href_enabled for the first row (default: false).
func AutoHrefEnabledFromDB(db *gorm.DB) bool {
	return SiteSettingOrDefault(db).AutoHrefEnabled
}

// ScraperAutoTagFromDB returns site_settings.scraper_auto_tag for the first row.
// Returns true when the row is missing or on read error (same default as scraper/bulk).
func ScraperAutoTagFromDB(db *gorm.DB) bool {
	return SiteSettingOrDefault(db).ScraperAutoTag
}

// PostImageStoreLimitsFromDB returns site_settings caps for stored post images (AI / scraper pipeline).
// Reads stored_image_max_width and stored_image_quality (not Cloudflare image_resize_* fields).
func PostImageStoreLimitsFromDB(db *gorm.DB) (maxLongestEdge int, webpQuality int) {
	s := SiteSettingOrDefault(db)
	if w := strings.TrimSpace(s.StoredImageMaxWidth); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			maxLongestEdge = n
		}
	}
	if q := strings.TrimSpace(s.StoredImageQuality); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 1 && n <= 100 {
			webpQuality = n
		}
	}
	return
}
