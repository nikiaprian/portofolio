package repositories

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

func logSearchLikePattern(raw string) (like string, ok bool) {
	q := strings.TrimSpace(raw)
	if q == "" {
		return "", false
	}
	q = strings.ReplaceAll(q, "%", "")
	q = strings.ReplaceAll(q, "_", "")
	q = strings.TrimSpace(q)
	if q == "" {
		return "", false
	}
	return "%" + q + "%", true
}

func logColumnLike(column, pat string) clause.Expression {
	return clause.Like{Column: clause.Column{Name: column}, Value: pat}
}

func logColumnLikePrefix(column, prefix string) clause.Expression {
	return logColumnLike(column, prefix+"%")
}

func logOrLike(columns []string, pat string) clause.Expression {
	exprs := make([]clause.Expression, len(columns))
	for i, col := range columns {
		exprs[i] = logColumnLike(col, pat)
	}
	return clause.OrConditions{Exprs: exprs}
}

func logInUint(column string, ids []uint) clause.Expression {
	vals := make([]interface{}, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	return clause.IN{Column: clause.Column{Name: column}, Values: vals}
}

func userIDsMatchingLogSearch(db *gorm.DB, pat string) ([]uint, error) {
	var ids []uint
	err := db.Model(&entity.User{}).
		Where(logOrLike([]string{"name", "email"}, pat)).
		Pluck("id", &ids).Error
	return ids, err
}

func categoryIDsMatchingLogSearch(db *gorm.DB, pat string) ([]uint, error) {
	var ids []uint
	err := db.Model(&entity.Category{}).
		Where(logColumnLike("name", pat)).
		Pluck("id", &ids).Error
	return ids, err
}

func applyBulkPostLogTextSearch(db *gorm.DB, q *gorm.DB, pat string) (*gorm.DB, error) {
	userIDs, err := userIDsMatchingLogSearch(db, pat)
	if err != nil {
		return nil, err
	}
	categoryIDs, err := categoryIDsMatchingLogSearch(db, pat)
	if err != nil {
		return nil, err
	}

	exprs := []clause.Expression{
		logOrLike([]string{"message", "title", "event", "provider_used"}, pat),
	}
	if len(userIDs) > 0 {
		exprs = append(exprs, logInUint("user_id", userIDs))
	}
	if len(categoryIDs) > 0 {
		exprs = append(exprs, logInUint("category_id", categoryIDs))
	}
	return q.Where(clause.OrConditions{Exprs: exprs}), nil
}

func applyManualPostAILogTextSearch(db *gorm.DB, q *gorm.DB, pat string) (*gorm.DB, error) {
	userIDs, err := userIDsMatchingLogSearch(db, pat)
	if err != nil {
		return nil, err
	}

	exprs := []clause.Expression{
		logOrLike([]string{"message", "title", "event", "provider_used", "provider_input"}, pat),
	}
	if len(userIDs) > 0 {
		exprs = append(exprs, logInUint("user_id", userIDs))
	}
	return q.Where(clause.OrConditions{Exprs: exprs}), nil
}
