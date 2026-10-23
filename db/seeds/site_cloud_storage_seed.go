package seeds

import (
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func seedSiteCloudStorage(db *gorm.DB) error {
	row := entity.DefaultSiteCloudStorage()
	return db.Create(&row).Error
}
