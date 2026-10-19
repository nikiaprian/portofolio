package alsoread

import (
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

// ApplyForPost strips frozen blocks, then re-injects them when the post is published and has a category.
// Draft / scheduled posts only get strips so editors do not accumulate stale boxes.
func ApplyForPost(db *gorm.DB, p *entity.Post) error {
	if p == nil || db == nil {
		return nil
	}
	stripped := Strip(p.Content)
	alsoReadOn, maxInserts, paragraphInterval := repositories.AlsoReadSettingsFromDB(db)
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
	p.Content = InjectFrozen(stripped, mode, related, maxInserts, paragraphInterval)
	return nil
}

// InjectForDisplay strips stale blocks and injects fresh "Baca juga" for public HTML rendering.
// It does not write to the database (used when stored content predates sync or settings changed).
func InjectForDisplay(db *gorm.DB, content string, postID, categoryID uint) (string, error) {
	if db == nil || postID == 0 {
		return Strip(content), nil
	}
	p := &entity.Post{
		ID:         postID,
		CategoryID: categoryID,
		Content:    content,
		Status:     "published",
	}
	if err := ApplyForPost(db, p); err != nil {
		return Strip(content), err
	}
	return p.Content, nil
}
