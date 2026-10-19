package repositories

import (
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// postTagsPivot is the post↔tag join table. Schema is created by GORM many2many on
// entity.Post / entity.Tag; this type exists only for explicit DELETEs (FK cleanup).
type postTagsPivot struct {
	PostID uint `gorm:"column:post_id"`
	TagID  uint `gorm:"column:tag_id"`
}

func (postTagsPivot) TableName() string {
	return "post_tags"
}

// deletePostsByIDsForTx removes post_tags rows then posts (must run inside the same tx).
func deletePostsByIDsForTx(tx *gorm.DB, ids []uint) error {
	for _, pid := range ids {
		if err := tx.Where("post_id = ?", pid).Delete(&postTagsPivot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("post_id = ?", pid).Delete(&entity.PostView{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&entity.Post{}, pid).Error; err != nil {
			return err
		}
	}
	return nil
}
