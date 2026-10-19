package alsoread

import (
	"time"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

const syncPublishedBatchSize = 100

// SyncPublishedContent updates all visible published posts when also_read_enabled toggles:
// strips blocks when disabled, or re-injects when enabled.
func SyncPublishedContent(db *gorm.DB, enabled bool) error {
	if db == nil {
		return nil
	}
	now := time.Now()
	var batch []entity.Post
	return db.Model(&entity.Post{}).
		Scopes(postutil.ScopeVisiblePublished("", now)).
		Select("id", "content", "status", "category_id").
		FindInBatches(&batch, syncPublishedBatchSize, func(tx *gorm.DB, _ int) error {
			postRepo := repositories.NewPostRepository(tx)
			for i := range batch {
				p := &batch[i]
				if enabled {
					if err := ApplyForPost(tx, p); err != nil {
						return err
					}
				} else {
					p.Content = Strip(p.Content)
				}
				// UpdateColumn only: plain Update bumps updated_at and makes all posts show "baru saja".
				if err := postRepo.UpdateContentOnly(p.ID, p.Content); err != nil {
					return err
				}
			}
			return nil
		}).Error
}
