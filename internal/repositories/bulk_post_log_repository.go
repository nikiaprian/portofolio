package repositories

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// BulkPostLogListFilter filters paginated bulk post audit rows.
type BulkPostLogListFilter struct {
	RequesterID uint
	WriterOnly  bool
	Level       string
	EventPrefix string
	Search      string
	Page        int
	Size        int
}

// BulkPostLogRepository lists bulk_post_logs with related user/category via Preload.
type BulkPostLogRepository interface {
	FindPaginated(filter BulkPostLogListFilter) ([]entity.BulkPostLog, int64, error)
	ClearAll() (int64, error)
}

type bulkPostLogRepository struct {
	db *gorm.DB
}

func NewBulkPostLogRepository(db *gorm.DB) BulkPostLogRepository {
	return &bulkPostLogRepository{db: db}
}

func (r *bulkPostLogRepository) applyFilter(filter BulkPostLogListFilter) (*gorm.DB, error) {
	q := r.db.Model(&entity.BulkPostLog{})

	if filter.WriterOnly && filter.RequesterID > 0 {
		q = q.Where(&entity.BulkPostLog{UserID: filter.RequesterID})
	}
	if lv := strings.ToLower(strings.TrimSpace(filter.Level)); lv != "" {
		q = q.Where(&entity.BulkPostLog{Level: lv})
	}
	if ev := strings.TrimSpace(filter.EventPrefix); ev != "" {
		q = q.Where(logColumnLikePrefix("event", ev))
	}
	if pat, ok := logSearchLikePattern(filter.Search); ok {
		var err error
		q, err = applyBulkPostLogTextSearch(r.db, q, pat)
		if err != nil {
			return nil, err
		}
	}
	return q, nil
}

func (r *bulkPostLogRepository) FindPaginated(filter BulkPostLogListFilter) ([]entity.BulkPostLog, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Size < 1 {
		filter.Size = 50
	}
	if filter.Size > 500 {
		filter.Size = 500
	}
	offset := (filter.Page - 1) * filter.Size

	base, err := r.applyFilter(filter)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []entity.BulkPostLog
	err = base.
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "email")
		}).
		Preload("Category", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Order("id DESC").
		Limit(filter.Size).
		Offset(offset).
		Find(&rows).Error
	return rows, total, err
}

func (r *bulkPostLogRepository) ClearAll() (int64, error) {
	res := r.db.Where(clause.Gt{Column: clause.Column{Name: "id"}, Value: 0}).Delete(&entity.BulkPostLog{})
	return res.RowsAffected, res.Error
}
