package repositories

import (
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// SiteCloudStorageRepository persists the singleton site_cloud_storage row.
type SiteCloudStorageRepository interface {
	Get() (*entity.SiteCloudStorage, error)
	Create(s *entity.SiteCloudStorage) error
	Save(s *entity.SiteCloudStorage) error
}

type siteCloudStorageRepository struct {
	db *gorm.DB
}

// NewSiteCloudStorageRepository constructs SiteCloudStorageRepository.
func NewSiteCloudStorageRepository(db *gorm.DB) SiteCloudStorageRepository {
	return &siteCloudStorageRepository{db: db}
}

func (r *siteCloudStorageRepository) Get() (*entity.SiteCloudStorage, error) {
	var s entity.SiteCloudStorage
	if err := r.db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *siteCloudStorageRepository) Create(s *entity.SiteCloudStorage) error {
	return r.db.Create(s).Error
}

func (r *siteCloudStorageRepository) Save(s *entity.SiteCloudStorage) error {
	return r.db.Save(s).Error
}
