package repositories

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func normalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func lookupCategoryIDBySlug(db *gorm.DB, slug string) (uint, bool, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return 0, false, nil
	}
	var cat entity.Category
	err := db.Model(&entity.Category{}).
		Select("id").
		Where("LOWER(TRIM(slug)) = ? AND enabled = ?", slug, true).
		Take(&cat).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return cat.ID, true, nil
}

func lookupTagBySlug(db *gorm.DB, slug string) (*entity.Tag, bool, error) {
	slug = normalizeSlug(slug)
	if slug == "" {
		return nil, false, nil
	}
	var tag entity.Tag
	err := db.Model(&entity.Tag{}).
		Where("LOWER(TRIM(slug)) = ?", slug).
		Take(&tag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &tag, true, nil
}

func lookupTagIDBySlug(db *gorm.DB, slug string) (uint, bool, error) {
	tag, ok, err := lookupTagBySlug(db, slug)
	if err != nil || !ok {
		return 0, ok, err
	}
	return tag.ID, true, nil
}

func postIDsForTag(db *gorm.DB, tagID uint) ([]uint, error) {
	if tagID == 0 {
		return nil, nil
	}
	var ids []uint
	err := db.Model(&postTagsPivot{}).
		Where(&postTagsPivot{TagID: tagID}).
		Pluck("post_id", &ids).Error
	return ids, err
}
