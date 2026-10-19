package repositories

import (
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// SiteLayoutRepository persists custom site layouts.
type SiteLayoutRepository interface {
	FindPaginated(page int, size int, q string) ([]entity.SiteLayout, int64, error)
	FindByID(id uint) (*entity.SiteLayout, error)
	Create(row *entity.SiteLayout) error
	Update(row *entity.SiteLayout) error
	Delete(id uint) error
	// SetUsedExclusive clears is_used on all layouts, then marks the given id as used.
	SetUsedExclusive(id uint) error
}

type siteLayoutRepository struct {
	db *gorm.DB
}

func NewSiteLayoutRepository(db *gorm.DB) SiteLayoutRepository {
	return &siteLayoutRepository{db: db}
}

func (r *siteLayoutRepository) FindPaginated(page int, size int, q string) ([]entity.SiteLayout, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	offset := (page - 1) * size
	q = strings.TrimSpace(q)

	base := r.db.Model(&entity.SiteLayout{})
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		base = base.Where("LOWER(layout_name) LIKE ?", like)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []entity.SiteLayout
	err := base.Order("updated_at desc").Limit(size).Offset(offset).Find(&list).Error
	return list, total, err
}

func (r *siteLayoutRepository) FindByID(id uint) (*entity.SiteLayout, error) {
	var row entity.SiteLayout
	if err := r.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *siteLayoutRepository) Create(row *entity.SiteLayout) error {
	return r.db.Create(row).Error
}

func (r *siteLayoutRepository) Update(row *entity.SiteLayout) error {
	return r.db.Save(row).Error
}

func (r *siteLayoutRepository) Delete(id uint) error {
	return r.db.Delete(&entity.SiteLayout{}, id).Error
}

func (r *siteLayoutRepository) SetUsedExclusive(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.SiteLayout{}).Where("1 = 1").Update("is_used", false).Error; err != nil {
			return err
		}
		res := tx.Model(&entity.SiteLayout{}).Where("id = ?", id).Update("is_used", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
