package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/publishedpost"
)

const postSelectColumnsNoView = "posts.id, posts.title, posts.slug, posts.content, posts.image, posts.image_origin, posts.image_origin_detail, posts.meta_desc, posts.meta_keywords, posts.source, posts.include_source, posts.status, posts.publish_at, posts.author_id, posts.category_id, posts.indexing_status, posts.indexed_at, posts.indexing_message, posts.created_at, posts.updated_at"

// post repository interface
type PostRepository interface {
	FindPaginated(page int, size int, q string, status string, categoryID uint, authorID uint) ([]entity.Post, int64, error)
	FindPaginatedByAuthor(page int, size int, authorID uint, q string, status string, categoryID uint, filterAuthorID uint) ([]entity.Post, int64, error)
	FindPublishedPaginated(page int, size int, skip int, categorySlug string, excludeCategorySlug string) ([]entity.Post, int64, error)
	FindPublishedPaginatedByCategoryID(page int, size int, skip int, categoryID uint) ([]entity.Post, int64, error)
	FindPublishedPaginatedByTagID(page int, size int, skip int, tagID uint) ([]entity.Post, int64, error)
	FindPublishedByTagID(tagID uint) ([]entity.Post, error)
	FindByID(id uint) (*entity.Post, error)
	FindPublishedBySlug(slug string) (*entity.Post, error)
	SlugExists(slug string, excludePostID uint) (bool, error)
	Create(p *entity.Post) error
	Update(p *entity.Post) error
	UpdatePreservingUpdatedAt(p *entity.Post, updatedAt time.Time) error
	// UpdateContentOnly updates content without touching updated_at (e.g. Baca juga bulk sync).
	UpdateContentOnly(id uint, content string) error
	BulkUpdateStatus(ids []uint, status string, authorID *uint) (int64, error)
	Delete(id uint) error
	// FindByCategoryIDForAssetCleanup returns posts in the category with fields needed to remove stored images (cover and content).
	FindByCategoryIDForAssetCleanup(categoryID uint) ([]entity.Post, error)
	// RecordUniqueView inserts (post_id, viewer_ip, view_date) if missing (UTC date YYYY-MM-DD).
	// It returns true when the row is newly inserted.
	RecordUniqueView(postID uint, viewerIP string, viewDateUTC string) (inserted bool, err error)
	// CountViewsForPost returns the number of unique viewer-day rows for the post.
	CountViewsForPost(postID uint) (int64, error)
	// FindPopularPublishedSince returns published posts ordered by view rows since cutoff.
	FindPopularPublishedSince(cutoff time.Time, limit int) ([]entity.Post, error)
	// FindPopularPublishedAllTime returns published posts ranked by total view rows (optional excludePostIDs).
	FindPopularPublishedAllTime(excludePostIDs []uint, limit int) ([]entity.Post, error)
	// FindPublishedInCategoryOrderedByViews lists published posts in a category (excluding one id), ordered by total views.
	FindPublishedInCategoryOrderedByViews(categoryID, excludePostID uint, limit int) ([]entity.Post, error)
	// PublishDueScheduledPosts sets status to published and clears publish_at for scheduled rows at or before cutoff.
	PublishDueScheduledPosts(cutoff time.Time) (int64, error)
	FindScheduledDue(cutoff time.Time, limit int) ([]entity.Post, error)
	// RefreshOldPublishedDatesBatch sets updated_at to now for up to limit oldest published posts whose date is <= cutoff (interval 0 / bulk).
	RefreshOldPublishedDatesBatch(cutoff, now time.Time, limit int) (int64, error)
	// RefreshOneOldPublishedDate updates updated_at to now for the single oldest published post whose date is <= cutoff (used with real-time interval between runs).
	RefreshOneOldPublishedDate(cutoff, now time.Time) (updated bool, err error)
	// TouchPublishedUpdatedAtBySlug sets updated_at for a published post with this slug (no-op if none).
	TouchPublishedUpdatedAtBySlug(slug string, now time.Time) (updated bool, err error)
}

// post repository struct
type postRepository struct {
	db *gorm.DB
}

// function to create a new post repository
func NewPostRepository(db *gorm.DB) PostRepository {
	return &postRepository{db: db}
}

// function to find paginated posts
func (r *postRepository) FindPaginated(page int, size int, q string, status string, categoryID uint, authorID uint) ([]entity.Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	offset := (page - 1) * size
	q = strings.TrimSpace(q)

	base := r.db.Model(&entity.Post{})
	if q != "" {
		like := "%" + q + "%"
		base = base.Where(
			"title LIKE ? OR slug LIKE ? OR status LIKE ?",
			like, like, like,
		)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "draft" || status == "published" || status == "scheduled" {
		base = base.Where(&entity.Post{Status: status})
	}
	if categoryID > 0 {
		base = base.Where(&entity.Post{CategoryID: categoryID})
	}
	if authorID > 0 {
		base = base.Where(&entity.Post{AuthorID: authorID})
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.Post
	query := base.
		Select("id", "title", "slug", "status", "publish_at", "author_id", "category_id", "indexing_status", "indexed_at", "indexing_message", "created_at", "updated_at").
		Preload("Author", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Preload("Category", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "slug")
		})
	err := orderAdminPostList(query, status).
		Order("id DESC").
		Limit(size).
		Offset(offset).
		Find(&list).Error
	return list, total, err
}

func (r *postRepository) FindPaginatedByAuthor(page int, size int, authorID uint, q string, status string, categoryID uint, filterAuthorID uint) ([]entity.Post, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if authorID == 0 {
		return []entity.Post{}, 0, nil
	}

	offset := (page - 1) * size
	q = strings.TrimSpace(q)

	base := r.db.Model(&entity.Post{}).Where(&entity.Post{AuthorID: authorID})
	if q != "" {
		like := "%" + q + "%"
		base = base.Where(
			"title LIKE ? OR slug LIKE ? OR status LIKE ?",
			like, like, like,
		)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "draft" || status == "published" || status == "scheduled" {
		base = base.Where(&entity.Post{Status: status})
	}
	if categoryID > 0 {
		base = base.Where(&entity.Post{CategoryID: categoryID})
	}
	if filterAuthorID > 0 {
		// Writer can only see their own posts even if a different author is requested.
		base = base.Where(&entity.Post{AuthorID: filterAuthorID})
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.Post
	query := base.
		Select("id", "title", "slug", "status", "publish_at", "author_id", "category_id", "indexing_status", "indexed_at", "indexing_message", "created_at", "updated_at").
		Preload("Author", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Preload("Category", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "slug")
		})
	err := orderAdminPostList(query, status).
		Order("id DESC").
		Limit(size).
		Offset(offset).
		Find(&list).Error
	return list, total, err
}

func orderAdminPostList(query *gorm.DB, status string) *gorm.DB {
	if status == "scheduled" {
		return query.
			Order("publish_at IS NULL ASC").
			Order("publish_at ASC")
	}
	if status == "" {
		return query.
			Order("CASE WHEN status = 'scheduled' THEN 0 ELSE 1 END ASC").
			Order("CASE WHEN status = 'scheduled' THEN publish_at END ASC").
			Order("COALESCE(updated_at, created_at) DESC")
	}
	return query.Order("COALESCE(updated_at, created_at) DESC")
}

// FindPublishedPaginated lists published posts only, newest first.
// skip offsets into the result set before applying page/size (e.g. skip=1 skips the newest post for a home feed).
// When categorySlug is non-empty, only posts in that category (by categories.slug) are returned.
// When excludeCategorySlug is non-empty, posts in that category are omitted from list results.
func (r *postRepository) FindPublishedPaginated(page int, size int, skip int, categorySlug string, excludeCategorySlug string) ([]entity.Post, int64, error) {
	base, ok, err := publishedPostsFilterQuery(r.db, categorySlug, excludeCategorySlug)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return []entity.Post{}, 0, nil
	}
	return paginatePublishedPosts(base, page, size, skip, func(q *gorm.DB) *gorm.DB {
		return q.Preload("Author").Preload("Category").Preload("Tags")
	})
}

func (r *postRepository) FindPublishedPaginatedByCategoryID(page int, size int, skip int, categoryID uint) ([]entity.Post, int64, error) {
	if categoryID == 0 {
		return []entity.Post{}, 0, nil
	}
	base := publishedPostsByCategoryID(r.db, categoryID)
	return paginatePublishedPosts(base, page, size, skip, func(q *gorm.DB) *gorm.DB {
		return q.Preload("Category").Preload("Category.Parent")
	})
}

func (r *postRepository) FindPublishedPaginatedByTagID(page int, size int, skip int, tagID uint) ([]entity.Post, int64, error) {
	if tagID == 0 {
		return []entity.Post{}, 0, nil
	}
	base, ok, err := publishedPostsByTagID(r.db, tagID)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return []entity.Post{}, 0, nil
	}
	return paginatePublishedPosts(base, page, size, skip, func(q *gorm.DB) *gorm.DB {
		return q.Select(postClientListColumns)
	})
}

func (r *postRepository) FindPublishedByTagID(tagID uint) ([]entity.Post, error) {
	posts, _, err := r.FindPublishedPaginatedByTagID(1, 1<<31-1, 0, tagID)
	return posts, err
}

// function to find a post by id
func (r *postRepository) FindByID(id uint) (*entity.Post, error) {
	var p entity.Post
	if err := r.db.Select(postSelectColumnsNoView).Preload("Author").Preload("Category").Preload("Tags").First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// function to find a published post by URL slug (public reader pages)
func (r *postRepository) FindPublishedBySlug(slug string) (*entity.Post, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var p entity.Post
	err := r.db.Select(postSelectColumnsNoView).Preload("Author").Preload("Category").Preload("Tags").
		Where(&entity.Post{Slug: slug}).
		Scopes(publishedpost.ScopeVisiblePublished("", time.Now())).
		Limit(1).
		Find(&p).Error
	if err != nil {
		return nil, err
	}
	if p.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &p, nil
}

// SlugExists reports whether another post already uses this slug.
// excludePostID skips that row (use the current post id on update; 0 on create).
func (r *postRepository) SlugExists(slug string, excludePostID uint) (bool, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false, nil
	}
	q := r.db.Model(&entity.Post{}).Where(&entity.Post{Slug: slug})
	if excludePostID > 0 {
		q = q.Not(&entity.Post{ID: excludePostID})
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// function to create a new post
func (r *postRepository) Create(p *entity.Post) error {
	return r.db.Create(p).Error
}

// function to update a post
func (r *postRepository) Update(p *entity.Post) error {
	// FindByID preloads Category/Author; GORM Save can sync BelongsTo from stale nested IDs and
	// silently revert category_id / author_id even after those fields were updated (see gorm#7596).
	return r.db.Omit("Category", "Author").Save(p).Error
}

func (r *postRepository) UpdateContentOnly(id uint, content string) error {
	if id == 0 {
		return nil
	}
	return r.db.Model(&entity.Post{}).Where(&entity.Post{ID: id}).UpdateColumn("content", content).Error
}

func (r *postRepository) UpdatePreservingUpdatedAt(p *entity.Post, updatedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Save still runs GORM's auto timestamp callback, so restore updated_at with
		// UpdateColumn after the row data and tag associations have been persisted.
		if err := tx.Omit("Category", "Author").Save(p).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.Post{}).Where(&entity.Post{ID: p.ID}).UpdateColumn("updated_at", updatedAt).Error; err != nil {
			return err
		}
		p.UpdatedAt = updatedAt
		return nil
	})
}

func (r *postRepository) BulkUpdateStatus(ids []uint, status string, authorID *uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	q := r.db.Model(&entity.Post{}).Where("id IN ?", ids)
	if authorID != nil {
		q = q.Where(&entity.Post{AuthorID: *authorID})
	}
	now := time.Now()
	if strings.EqualFold(status, "published") {
		res := q.Updates(map[string]any{
			"status":     "published",
			"publish_at": nil,
			"updated_at": now,
		})
		return res.RowsAffected, res.Error
	}
	res := q.Updates(map[string]any{
		"status":     "draft",
		"publish_at": nil,
		"updated_at": now,
	})
	return res.RowsAffected, res.Error
}

func (r *postRepository) PublishDueScheduledPosts(cutoff time.Time) (int64, error) {
	res := r.db.Model(&entity.Post{}).
		Where(&entity.Post{Status: "scheduled"}).
		Where("publish_at IS NOT NULL AND publish_at <= ?", cutoff).
		Updates(map[string]any{
			"status":     "published",
			"publish_at": nil,
			"updated_at": time.Now().UTC(),
		})
	return res.RowsAffected, res.Error
}

func (r *postRepository) FindScheduledDue(cutoff time.Time, limit int) ([]entity.Post, error) {
	if limit <= 0 {
		limit = 100
	}
	var out []entity.Post
	err := r.db.
		Select(postSelectColumnsNoView).
		Where(&entity.Post{Status: "scheduled"}).
		Where("publish_at IS NOT NULL AND publish_at <= ?", cutoff).
		Order("publish_at ASC").
		Limit(limit).
		Find(&out).Error
	return out, err
}

func (r *postRepository) RefreshOldPublishedDatesBatch(cutoff, now time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 200
	}
	ts := now.UTC()
	var ids []uint
	if err := r.db.Model(&entity.Post{}).
		Where(&entity.Post{Status: "published"}).
		Where("COALESCE(updated_at, created_at) <= ?", cutoff).
		Order("COALESCE(updated_at, created_at) ASC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.Model(&entity.Post{}).
		Where("id IN ?", ids).
		Update("updated_at", ts)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *postRepository) RefreshOneOldPublishedDate(cutoff, now time.Time) (bool, error) {
	ts := now.UTC()
	var ids []uint
	if err := r.db.Model(&entity.Post{}).
		Where(&entity.Post{Status: "published"}).
		Where("COALESCE(updated_at, created_at) <= ?", cutoff).
		Order("COALESCE(updated_at, created_at) ASC").
		Limit(1).
		Pluck("id", &ids).Error; err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}
	res := r.db.Model(&entity.Post{}).
		Where(&entity.Post{ID: ids[0]}).
		Update("updated_at", ts)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *postRepository) TouchPublishedUpdatedAtBySlug(slug string, now time.Time) (bool, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false, nil
	}
	ts := now.UTC()
	res := r.db.Model(&entity.Post{}).
		Where(&entity.Post{Slug: slug, Status: "published"}).
		Update("updated_at", ts)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// function to delete a post
func (r *postRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return deletePostsByIDsForTx(tx, []uint{id})
	})
}

func (r *postRepository) FindByCategoryIDForAssetCleanup(categoryID uint) ([]entity.Post, error) {
	if categoryID == 0 {
		return nil, nil
	}
	var list []entity.Post
	err := r.db.Model(&entity.Post{}).
		Select("id", "image", "content").
		Where(&entity.Post{CategoryID: categoryID}).
		Find(&list).Error
	return list, err
}

func (r *postRepository) RecordUniqueView(postID uint, viewerIP string, viewDateUTC string) (bool, error) {
	return NewPostViewRepository(r.db).RecordUniqueView(postID, viewerIP, viewDateUTC)
}

func (r *postRepository) CountViewsForPost(postID uint) (int64, error) {
	return NewPostViewRepository(r.db).CountForPost(postID)
}

func (r *postRepository) postsByViewRanks(ranks []PostViewRank, limit int) ([]entity.Post, error) {
	if len(ranks) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = len(ranks)
	}
	ids := make([]uint, 0, len(ranks))
	viewsByID := make(map[uint]int64, len(ranks))
	for _, rank := range ranks {
		if rank.PostID == 0 {
			continue
		}
		ids = append(ids, rank.PostID)
		viewsByID[rank.PostID] = rank.Views
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var posts []entity.Post
	err := publishedPostsQuery(r.db).
		Select(postSelectColumnsNoView).
		Preload("Category").
		Where("id IN ?", ids).
		Find(&posts).Error
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]entity.Post, len(posts))
	for _, p := range posts {
		p.View = viewsByID[p.ID]
		byID[p.ID] = p
	}
	out := make([]entity.Post, 0, limit)
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, p)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (r *postRepository) FindPublishedInCategoryOrderedByViews(categoryID, excludePostID uint, limit int) ([]entity.Post, error) {
	if categoryID == 0 || limit <= 0 {
		return nil, nil
	}
	ranks, err := NewPostViewRepository(r.db).RankInCategory(categoryID, excludePostID, limit)
	if err != nil {
		return nil, err
	}
	return r.postsByViewRanks(ranks, limit)
}

func (r *postRepository) FindPopularPublishedSince(cutoff time.Time, limit int) ([]entity.Post, error) {
	if limit <= 0 {
		limit = 5
	}

	ranks, err := NewPostViewRepository(r.db).RankSince(cutoff, limit)
	if err != nil {
		return nil, err
	}
	return r.postsByViewRanks(ranks, limit)
}

func (r *postRepository) FindPopularPublishedAllTime(excludePostIDs []uint, limit int) ([]entity.Post, error) {
	if limit <= 0 {
		return nil, nil
	}
	ranks, err := NewPostViewRepository(r.db).RankPublishedAllTime(excludePostIDs, limit)
	if err != nil {
		return nil, err
	}
	return r.postsByViewRanks(ranks, limit)
}
