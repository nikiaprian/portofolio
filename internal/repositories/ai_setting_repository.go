package repositories

import (
	"errors"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// AISettingRepository persists the singleton ai_setting row (provider keys and models).
type AISettingRepository interface {
	GetOrCreate() (*entity.AISetting, error)
	Save(a *entity.AISetting) error
}

type aiSettingRepository struct {
	db *gorm.DB
}

// NewAISettingRepository constructs an AISettingRepository.
func NewAISettingRepository(db *gorm.DB) AISettingRepository {
	return &aiSettingRepository{db: db}
}

func (r *aiSettingRepository) GetOrCreate() (*entity.AISetting, error) {
	var row entity.AISetting
	// ai_setting is intended to be a singleton table, but in case multiple rows exist
	// (e.g. manual DB edits or old bugs), always pick the most recently updated row
	// to keep behavior deterministic and aligned with the last-saved admin settings.
	err := r.db.Order("updated_at desc").Order("id desc").First(&row).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row = entity.AISetting{}
	if err := r.db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *aiSettingRepository) Save(a *entity.AISetting) error {
	return r.db.Save(a).Error
}
