package publicshelf

import "gorm.io/gorm"

// Active reports whether listings and sitemaps are limited to a topic shelf.
// This news branch indexes every published article, so the shelf stays off.
func Active(layout string) bool {
	return false
}

// Matches reports whether a post belongs on the shelf.
// Every article is indexed on this branch.
func Matches(title, slug string) bool {
	return true
}

// Scope limits a posts query to the shelf. This branch does not limit it.
func Scope(prefix string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db
	}
}

// LayoutActive reads the first site_settings row on a fresh session.
func LayoutActive(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	var row struct {
		HomepageLayout string `gorm:"column:homepage_layout"`
	}
	err := db.Session(&gorm.Session{NewDB: true}).
		Table("site_settings").
		Select("homepage_layout").
		Order("id ASC").
		Limit(1).
		Take(&row).Error
	if err != nil {
		return false
	}
	return Active(row.HomepageLayout)
}
