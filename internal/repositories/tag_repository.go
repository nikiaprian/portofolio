package repositories

import (
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// TagRepository persists tags.
type TagRepository interface {
	FindPaginated(page int, size int, q string) ([]entity.Tag, int64, error)
	FindByID(id uint) (*entity.Tag, error)
	FindBySlug(slug string) (*entity.Tag, error)
	FindIDBySlug(slug string) (uint, bool, error)
	PluckNamesOrderedByIDs(ids []uint) ([]string, error)
	FindByIDs(ids []uint) ([]entity.Tag, error)
	FindByNamesCaseInsensitive(names []string) ([]entity.Tag, error)
	// FindAllForAutoLink returns up to limit tags (longer names first) for in-body autolinking.
	FindAllForAutoLink(limit int) ([]entity.Tag, error)
	Create(t *entity.Tag) error
	Update(t *entity.Tag) error
	Delete(id uint) error
}

// tag repository struct
type tagRepository struct {
	db *gorm.DB
}

// function to create a new tag repository
func NewTagRepository(db *gorm.DB) TagRepository {
	return &tagRepository{db: db}
}

// function to find paginated tags
func (r *tagRepository) FindPaginated(page int, size int, q string) ([]entity.Tag, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	offset := (page - 1) * size
	q = strings.TrimSpace(q)

	base := r.db.Model(&entity.Tag{})
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		base = base.Where(
			"LOWER(name) LIKE ? OR LOWER(slug) LIKE ?",
			like, like,
		)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.Tag
	err := base.Order("updated_at desc").Limit(size).Offset(offset).Find(&list).Error
	return list, total, err
}

// function to find a tag by id
func (r *tagRepository) FindByID(id uint) (*entity.Tag, error) {
	var t entity.Tag
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tagRepository) FindBySlug(slug string) (*entity.Tag, error) {
	tag, ok, err := lookupTagBySlug(r.db, slug)
	if err != nil || !ok {
		if err != nil {
			return nil, err
		}
		return nil, gorm.ErrRecordNotFound
	}
	return tag, nil
}

func (r *tagRepository) FindIDBySlug(slug string) (uint, bool, error) {
	return lookupTagIDBySlug(r.db, slug)
}

// PluckNamesOrderedByIDs returns tag names sorted by id for the given ids.
func (r *tagRepository) PluckNamesOrderedByIDs(ids []uint) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var names []string
	if err := r.db.Model(&entity.Tag{}).Where("id IN ?", ids).Order("id ASC").Pluck("name", &names).Error; err != nil {
		return nil, err
	}
	return names, nil
}

// FindByIDs returns tags for the given ids (nil slice if ids is empty).
func (r *tagRepository) FindByIDs(ids []uint) ([]entity.Tag, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []entity.Tag
	if err := r.db.Where("id IN ?", ids).Order("id ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// FindByNamesCaseInsensitive returns tags whose LOWER(TRIM(name)) matches any trimmed name (case-insensitive). Tokens shorter than 2 runes are ignored.
func (r *tagRepository) FindByNamesCaseInsensitive(names []string) ([]entity.Tag, error) {
	lowered := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if utf8.RuneCountInString(n) < 2 {
			continue
		}
		k := strings.ToLower(n)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		lowered = append(lowered, k)
	}
	if len(lowered) == 0 {
		return nil, nil
	}
	var out []entity.Tag
	if err := r.db.Model(&entity.Tag{}).Where("LOWER(TRIM(name)) IN ?", lowered).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// FindAllForAutoLink returns tags with name length ≥ 2, ordered by longest name first (better overlap handling), capped by limit.
func (r *tagRepository) FindAllForAutoLink(limit int) ([]entity.Tag, error) {
	if limit <= 0 {
		limit = 250
	}
	var out []entity.Tag
	err := r.db.Model(&entity.Tag{}).
		Select("id", "name", "slug").
		Where("CHAR_LENGTH(name) >= ?", 2).
		Order("CHAR_LENGTH(name) DESC, id ASC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

// function to create a new tag
func (r *tagRepository) Create(t *entity.Tag) error {
	return r.db.Create(t).Error
}

// function to update a tag
func (r *tagRepository) Update(t *entity.Tag) error {
	return r.db.Save(t).Error
}

// function to delete a tag
func (r *tagRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Detach from posts first — pivot rows reference tags.id.
		if err := tx.Where(&postTagsPivot{TagID: id}).Delete(&postTagsPivot{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.Tag{}, id).Error
	})
}
