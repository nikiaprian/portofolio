package seeds

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func seedSiteConnection(db *gorm.DB) error {
	siteConn := entity.SiteConnection{}
	return db.Create(&siteConn).Error
}
