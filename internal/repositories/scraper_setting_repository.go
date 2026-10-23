package repositories

import (
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// ScraperSettingRepository persists the singleton scraper_settings row.
type ScraperSettingRepository interface {
	Get() (*entity.ScraperSetting, error)
	Create(s *entity.ScraperSetting) error
	Save(s *entity.ScraperSetting) error
}

type scraperSettingRepository struct {
	db *gorm.DB
}

// NewScraperSettingRepository constructs ScraperSettingRepository.
func NewScraperSettingRepository(db *gorm.DB) ScraperSettingRepository {
	return &scraperSettingRepository{db: db}
}

func (r *scraperSettingRepository) Get() (*entity.ScraperSetting, error) {
	var s entity.ScraperSetting
	if err := r.db.Order("id desc").First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *scraperSettingRepository) Create(s *entity.ScraperSetting) error {
	return r.db.Create(s).Error
}

func (r *scraperSettingRepository) Save(s *entity.ScraperSetting) error {
	return r.db.Save(s).Error
}
