package repositories

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// ManualPostAILogListFilter filters paginated manual post AI audit rows.
type ManualPostAILogListFilter struct {
	RequesterID uint
	WriterOnly  bool
	Level       string
	EventPrefix string
	Search      string
	Page        int
	Size        int
}

// ManualPostAILogRepository lists manual_post_ai_logs with related user via Preload.
type ManualPostAILogRepository interface {
	FindPaginated(filter ManualPostAILogListFilter) ([]entity.ManualPostAILog, int64, error)
	ClearAll() (int64, error)
}

type manualPostAILogRepository struct {
	db *gorm.DB
}

func NewManualPostAILogRepository(db *gorm.DB) ManualPostAILogRepository {
	return &manualPostAILogRepository{db: db}
}

func (r *manualPostAILogRepository) applyFilter(filter ManualPostAILogListFilter) (*gorm.DB, error) {
	q := r.db.Model(&entity.ManualPostAILog{})

	if filter.WriterOnly && filter.RequesterID > 0 {
		q = q.Where(&entity.ManualPostAILog{UserID: filter.RequesterID})
	}
	if lv := strings.ToLower(strings.TrimSpace(filter.Level)); lv != "" {
		q = q.Where(&entity.ManualPostAILog{Level: lv})
	}
	if ev := strings.TrimSpace(filter.EventPrefix); ev != "" {
		q = q.Where(logColumnLikePrefix("event", ev))
	}
	if pat, ok := logSearchLikePattern(filter.Search); ok {
		var err error
		q, err = applyManualPostAILogTextSearch(r.db, q, pat)
		if err != nil {
			return nil, err
		}
	}
	return q, nil
}

func (r *manualPostAILogRepository) FindPaginated(filter ManualPostAILogListFilter) ([]entity.ManualPostAILog, int64, error) {
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

	var rows []entity.ManualPostAILog
	err = base.
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "email")
		}).
		Order("id DESC").
		Limit(filter.Size).
		Offset(offset).
		Find(&rows).Error
	return rows, total, err
}

func (r *manualPostAILogRepository) ClearAll() (int64, error) {
	res := r.db.Where(clause.Gt{Column: clause.Column{Name: "id"}, Value: 0}).Delete(&entity.ManualPostAILog{})
	return res.RowsAffected, res.Error
}
