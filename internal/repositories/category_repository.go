package repositories

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post"
)

// CategoryRepository persists categories.
type CategoryRepository interface {
	FindPaginated(page int, size int, q string, onlyEnabled bool) ([]entity.Category, int64, error)
	FindByID(id uint) (*entity.Category, error)
	FindIDBySlug(slug string) (uint, bool, error)
	SitemapRows() ([]CategorySitemapRow, error)
	CountChildren(parentID uint) (int64, error)
	Create(c *entity.Category) error
	Update(c *entity.Category) error
	Delete(id uint) error
}

// CategorySitemapRow is one category entry for sitemap.xml (slug + latest published post activity).
type CategorySitemapRow struct {
	Slug      string
	UpdatedAt time.Time
}

// category repository struct
type categoryRepository struct {
	db *gorm.DB
}

// function to create a new category repository
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

// function to find paginated categories
func (r *categoryRepository) FindPaginated(page int, size int, q string, onlyEnabled bool) ([]entity.Category, int64, error) {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		return normalizeCategorySortTx(tx)
	}); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	offset := (page - 1) * size
	q = strings.TrimSpace(q)

	base := r.db.Model(&entity.Category{})
	if onlyEnabled {
		base = base.Where("enabled = ?", true)
	}
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		base = base.Where(
			"LOWER(name) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(description) LIKE ? OR LOWER(title) LIKE ? OR LOWER(keyword) LIKE ?",
			like, like, like, like, like,
		)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.Category
	err := base.Preload("Parent").Order("sort asc").Order("updated_at desc").Limit(size).Offset(offset).Find(&list).Error
	return list, total, err
}

// function to find a category by id
func (r *categoryRepository) FindByID(id uint) (*entity.Category, error) {
	var c entity.Category
	if err := r.db.Preload("Parent").First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepository) FindIDBySlug(slug string) (uint, bool, error) {
	return lookupCategoryIDBySlug(r.db, slug)
}

func (r *categoryRepository) SitemapRows() ([]CategorySitemapRow, error) {
	var cats []entity.Category
	if err := r.db.Model(&entity.Category{}).
		Select("id", "slug").
		Where("enabled = ?", true).
		Order("slug ASC").
		Find(&cats).Error; err != nil {
		return nil, err
	}
	type latestByCategory struct {
		CategoryID uint      `gorm:"column:category_id"`
		UpdatedAt  time.Time `gorm:"column:updated_at"`
	}
	var latest []latestByCategory
	if err := r.db.Model(&entity.Post{}).
		Select("category_id, MAX(COALESCE(updated_at, created_at)) AS updated_at").
		Scopes(postutil.ScopeVisiblePublished("", time.Now())).
		Group("category_id").
		Scan(&latest).Error; err != nil {
		return nil, err
	}
	updatedByCategory := make(map[uint]time.Time, len(latest))
	for _, row := range latest {
		updatedByCategory[row.CategoryID] = row.UpdatedAt
	}
	out := make([]CategorySitemapRow, len(cats))
	for i, cat := range cats {
		out[i] = CategorySitemapRow{
			Slug:      cat.Slug,
			UpdatedAt: updatedByCategory[cat.ID],
		}
	}
	return out, nil
}

func (r *categoryRepository) CountChildren(parentID uint) (int64, error) {
	var n int64
	err := r.db.Model(&entity.Category{}).Where(&entity.Category{ParentID: &parentID}).Count(&n).Error
	return n, err
}

// function to create a new category
func (r *categoryRepository) Create(c *entity.Category) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var total int64
		if err := tx.Model(&entity.Category{}).Count(&total).Error; err != nil {
			return err
		}
		maxSort := int(total) + 1
		targetSort := c.Sort
		if targetSort < 1 || targetSort > maxSort {
			targetSort = maxSort
		}
		if err := shiftCategorySortForInsertTx(tx, targetSort); err != nil {
			return err
		}
		c.Sort = targetSort
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		return normalizeCategorySortTx(tx)
	})
}

// function to update a category
func (r *categoryRepository) Update(c *entity.Category) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing entity.Category
		if err := tx.First(&existing, c.ID).Error; err != nil {
			return err
		}

		var total int64
		if err := tx.Model(&entity.Category{}).Count(&total).Error; err != nil {
			return err
		}
		targetSort := c.Sort
		if targetSort < 1 {
			targetSort = existing.Sort
		}
		if targetSort < 1 {
			targetSort = 1
		}
		if targetSort > int(total) {
			targetSort = int(total)
		}
		if targetSort != existing.Sort {
			if err := shiftCategorySortForMoveTx(tx, existing.ID, existing.Sort, targetSort); err != nil {
				return err
			}
		}
		c.Sort = targetSort
		if err := tx.Save(c).Error; err != nil {
			return err
		}
		return normalizeCategorySortTx(tx)
	})
}

// function to delete a category
func (r *categoryRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing entity.Category
		if err := tx.First(&existing, id).Error; err != nil {
			return err
		}
		var postIDs []uint
		if err := tx.Model(&entity.Post{}).Where(&entity.Post{CategoryID: id}).Pluck("id", &postIDs).Error; err != nil {
			return err
		}
		// Posts reference categories (fk_categories_posts); remove them first (and post_tags).
		if err := deletePostsByIDsForTx(tx, postIDs); err != nil {
			return err
		}
		if err := tx.Delete(&entity.Category{}, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.Category{}).
			Where("sort > ?", existing.Sort).
			Update("sort", gorm.Expr("sort - 1")).Error; err != nil {
			return err
		}
		return normalizeCategorySortTx(tx)
	})
}

func shiftCategorySortForInsertTx(tx *gorm.DB, sort int) error {
	if sort < 1 {
		return nil
	}
	return tx.Model(&entity.Category{}).
		Where("sort >= ?", sort).
		Update("sort", gorm.Expr("sort + 1")).Error
}

func shiftCategorySortForMoveTx(tx *gorm.DB, id uint, oldSort int, newSort int) error {
	if oldSort < 1 || newSort < 1 || oldSort == newSort {
		return nil
	}
	if newSort < oldSort {
		return tx.Model(&entity.Category{}).
			Where("id <> ? AND sort >= ? AND sort < ?", id, newSort, oldSort).
			Update("sort", gorm.Expr("sort + 1")).Error
	}
	return tx.Model(&entity.Category{}).
		Where("id <> ? AND sort > ? AND sort <= ?", id, oldSort, newSort).
		Update("sort", gorm.Expr("sort - 1")).Error
}

func normalizeCategorySortTx(tx *gorm.DB) error {
	var rows []struct {
		ID uint
	}
	if err := tx.Model(&entity.Category{}).
		Select("id").
		Order("CASE WHEN sort < 1 THEN 1 ELSE sort END ASC").
		Order("updated_at DESC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return err
	}
	for idx, row := range rows {
		nextSort := idx + 1
		if err := tx.Model(&entity.Category{}).
			Where("id = ? AND sort <> ?", row.ID, nextSort).
			Update("sort", nextSort).Error; err != nil {
			return err
		}
	}
	return nil
}
