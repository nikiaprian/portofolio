package repositories

import (
	"time"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/publicshelf"
	"github.com/nikiaprian/tirtayasa/internal/helper/publishedpost"
)

const postClientListColumns = "id, title, slug, content, image, category_id, updated_at, created_at"

func publishedPostsQuery(db *gorm.DB) *gorm.DB {
	q := db.Model(&entity.Post{}).Scopes(publishedpost.ScopeVisiblePublished("", time.Now()))
	if publicshelf.LayoutActive(db) {
		q = q.Scopes(publicshelf.Scope(""))
	}
	return q
}

func publishedPostsByCategoryID(db *gorm.DB, categoryID uint) *gorm.DB {
	q := publishedPostsQuery(db)
	if categoryID > 0 {
		q = q.Where(&entity.Post{CategoryID: categoryID})
	}
	return q
}

func publishedPostsByTagID(db *gorm.DB, tagID uint) (*gorm.DB, bool, error) {
	postIDs, err := postIDsForTag(db, tagID)
	if err != nil {
		return nil, false, err
	}
	if len(postIDs) == 0 {
		return nil, false, nil
	}
	return publishedPostsQuery(db).Where("id IN ?", postIDs), true, nil
}

// publishedPostsFilterQuery builds a published-post query filtered by category slug and/or exclude slug.
// ok is false when categorySlug is set but does not match any category (empty result set).
func publishedPostsFilterQuery(db *gorm.DB, categorySlug, excludeCategorySlug string) (*gorm.DB, bool, error) {
	categorySlug = normalizeSlug(categorySlug)
	excludeCategorySlug = normalizeSlug(excludeCategorySlug)

	q := publishedPostsQuery(db)

	if categorySlug != "" {
		categoryID, ok, err := lookupCategoryIDBySlug(db, categorySlug)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		q = q.Where(&entity.Post{CategoryID: categoryID})
	}

	if excludeCategorySlug != "" {
		excludeID, ok, err := lookupCategoryIDBySlug(db, excludeCategorySlug)
		if err != nil {
			return nil, false, err
		}
		if ok {
			q = q.Not(&entity.Post{CategoryID: excludeID})
		}
	}

	return q, true, nil
}

func paginatePublishedPosts(
	base *gorm.DB,
	page, size, skip int,
	preloads func(*gorm.DB) *gorm.DB,
) ([]entity.Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if skip < 0 {
		skip = 0
	}
	offset := skip + (page-1)*size

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	q := base.
		Select(postSelectColumnsNoView).
		Order("updated_at DESC").
		Limit(size).
		Offset(offset)
	if preloads != nil {
		q = preloads(q)
	}

	var list []entity.Post
	if err := q.Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
