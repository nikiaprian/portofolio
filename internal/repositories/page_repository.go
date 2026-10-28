package repositories

import (
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// page repository interface
type PageRepository interface {
	FindPaginated(page int, size int, q string) ([]entity.Page, int64, error)
	FindPublishedPaginated(page int, size int, skip int) ([]entity.Page, int64, error)
	FindByID(id uint) (*entity.Page, error)
	FindPublishedBySlug(slug string) (*entity.Page, error)
	SlugExists(slug string, excludePageID uint) (bool, error)
	Create(p *entity.Page) error
	Update(p *entity.Page) error
	BulkUpdateStatus(ids []uint, status string) (int64, error)
	Delete(id uint) error
}

// page repository struct
type pageRepository struct {
	db *gorm.DB
}

// function to create a new page repository
func NewPageRepository(db *gorm.DB) PageRepository {
	return &pageRepository{db: db}
}

// function to find paginated pages
func (r *pageRepository) FindPaginated(page int, size int, q string) ([]entity.Page, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	offset := (page - 1) * size
	q = strings.TrimSpace(q)

	base := r.db.Model(&entity.Page{})
	if q != "" {
		like := "%" + q + "%"
		base = base.Where(
			"title LIKE ? OR slug LIKE ?",
			like, like,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.Page
	err := base.
		Select("id", "title", "slug", "status", "indexing_status", "indexed_at", "indexing_message", "created_at", "updated_at").
		Order("updated_at desc").
		Limit(size).
		Offset(offset).
		Find(&list).Error
	return list, total, err
}

// FindPublishedPaginated lists published pages only, newest first.
// skip offsets into the result set before applying page/size (e.g. skip=1 skips the newest page for a home feed).
func (r *pageRepository) FindPublishedPaginated(page int, size int, skip int) ([]entity.Page, int64, error) {
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
	base := r.db.Model(&entity.Page{}).Where("status = ?", "published")

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.Page
	err := r.db.
		Where("status = ?", "published").
		Order("updated_at DESC").
		Limit(size).
		Offset(offset).
		Find(&list).Error
	return list, total, err
}

// function to find a page by id
func (r *pageRepository) FindByID(id uint) (*entity.Page, error) {
	var p entity.Page
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// function to find a published page by URL slug (public reader pages)
func (r *pageRepository) FindPublishedBySlug(slug string) (*entity.Page, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var p entity.Page
	err := r.db.
		Where("slug = ? AND status = ?", slug, "published").
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SlugExists reports whether another page already uses this slug.
// excludePageID skips that row (use the current page id on update; 0 on create).
func (r *pageRepository) SlugExists(slug string, excludePageID uint) (bool, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return false, nil
	}
	q := r.db.Model(&entity.Page{}).Where("slug = ?", slug)
	if excludePageID > 0 {
		q = q.Where("id <> ?", excludePageID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// function to create a new page
func (r *pageRepository) Create(p *entity.Page) error {
	return r.db.Create(p).Error
}

// function to update a page
func (r *pageRepository) Update(p *entity.Page) error {
	return r.db.Save(p).Error
}

func (r *pageRepository) BulkUpdateStatus(ids []uint, status string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	st := strings.ToLower(strings.TrimSpace(status))
	if st != "published" {
		st = "draft"
	}
	res := r.db.Model(&entity.Page{}).Where("id IN ?", ids).Update("status", st)
	return res.RowsAffected, res.Error
}

// function to delete a page
func (r *pageRepository) Delete(id uint) error {
	return r.db.Delete(&entity.Page{}, id).Error
}
