package repositories

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// SiteConnectionRepository persists the singleton site_connections row.
type SiteConnectionRepository interface {
	Get() (*entity.SiteConnection, error)
	Create(s *entity.SiteConnection) error
	Save(s *entity.SiteConnection) error
}

type siteConnectionRepository struct {
	db *gorm.DB
}

// NewSiteConnectionRepository constructs SiteConnectionRepository.
func NewSiteConnectionRepository(db *gorm.DB) SiteConnectionRepository {
	return &siteConnectionRepository{db: db}
}

func (r *siteConnectionRepository) Get() (*entity.SiteConnection, error) {
	var s entity.SiteConnection
	if err := r.db.First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *siteConnectionRepository) Create(s *entity.SiteConnection) error {
	return r.db.Create(s).Error
}

func (r *siteConnectionRepository) Save(s *entity.SiteConnection) error {
	return r.db.Save(s).Error
}
