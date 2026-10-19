package repositories

import (
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"gorm.io/gorm"
)

type SiteWebImageProviderRepository interface {
	List() ([]entity.SiteWebImageProvider, error)
	GetByProvider(provider string) (*entity.SiteWebImageProvider, error)
	Save(provider *entity.SiteWebImageProvider) error
	ReplaceAll(providers []entity.SiteWebImageProvider) error
}

type siteWebImageProviderRepository struct {
	db *gorm.DB
}

func NewSiteWebImageProviderRepository(db *gorm.DB) SiteWebImageProviderRepository {
	return &siteWebImageProviderRepository{db: db}
}

func (r *siteWebImageProviderRepository) List() ([]entity.SiteWebImageProvider, error) {
	var rows []entity.SiteWebImageProvider
	err := r.db.Order("provider ASC").Find(&rows).Error
	return rows, err
}

func (r *siteWebImageProviderRepository) GetByProvider(provider string) (*entity.SiteWebImageProvider, error) {
	var row entity.SiteWebImageProvider
	if err := r.db.Where("provider = ?", provider).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *siteWebImageProviderRepository) Save(provider *entity.SiteWebImageProvider) error {
	return r.db.Save(provider).Error
}

func (r *siteWebImageProviderRepository) ReplaceAll(providers []entity.SiteWebImageProvider) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range providers {
			var existing entity.SiteWebImageProvider
			err := tx.Where("provider = ?", providers[i].Provider).First(&existing).Error
			if err == nil {
				existing.Enabled = providers[i].Enabled
				existing.CredentialsJSON = providers[i].CredentialsJSON
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if err := tx.Create(&providers[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
