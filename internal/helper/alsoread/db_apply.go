package alsoread

import (
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
)

// ApplyForPost strips frozen blocks, then re-injects them when the post is published and has a category.
// Draft / scheduled posts only get strips so editors do not accumulate stale boxes.
func ApplyForPost(db *gorm.DB, p *entity.Post) error {
	if p == nil || db == nil {
		return nil
	}
	stripped := Strip(p.Content)
	alsoReadOn, maxInserts := repositories.AlsoReadSettingsFromDB(db)
	if !alsoReadOn {
		p.Content = stripped
		return nil
	}
	st := strings.ToLower(strings.TrimSpace(p.Status))
	if st != "published" {
		p.Content = stripped
		return nil
	}
	if p.CategoryID == 0 {
		p.Content = stripped
		return nil
	}
	repo := repositories.NewPostRepository(db)
	related, err := repo.FindPublishedInCategoryOrderedByViews(p.CategoryID, p.ID, relatedFetchLimitFor(maxInserts))
	if err != nil {
		return err
	}
	mode := repositories.ArticleSlugModeFromDB(db)
	p.Content = InjectFrozen(stripped, mode, related, maxInserts)
	return nil
}
