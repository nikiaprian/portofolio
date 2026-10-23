package seeds

import (
	"github.com/nikiaprian/tirtayasa/internal/entity"
	"gorm.io/gorm"
)

func seedSiteWebImageProviders(db *gorm.DB) error {
	rows := entity.DefaultSiteWebImageProviders()
	for i := range rows {
		var existing entity.SiteWebImageProvider
		err := db.Where("provider = ?", rows[i].Provider).First(&existing).Error
		if err == nil {
			continue
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err := db.Create(&rows[i]).Error; err != nil {
			return err
		}
	}
	return nil
}
