package seeds

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedSiteCloudStorage(db *gorm.DB) error {
	row := entity.DefaultSiteCloudStorage()
	return db.Create(&row).Error
}
