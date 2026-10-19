package postutil

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ScopeVisiblePublished limits rows to published posts whose category is missing or enabled.
// Uses NOT EXISTS (not a JOIN) so SELECT/ORDER/WHERE on posts columns stay unambiguous.
// The now parameter is retained for call-site compatibility; it is not used for filtering.
// Use tablePrefix "" for the posts table without alias, or "posts" when joining under that alias.
func ScopeVisiblePublished(tablePrefix string, now time.Time) func(*gorm.DB) *gorm.DB {
	_ = now
	return func(db *gorm.DB) *gorm.DB {
		p := strings.TrimSpace(tablePrefix)
		if p == "" {
			p = "posts"
		}
		if !strings.HasSuffix(p, ".") {
			p += "."
		}
		// Equivalent to LEFT JOIN categories … WHERE categories.id IS NULL OR categories.enabled:
		// hide only when a matching category row exists and is disabled.
		return db.
			Where(p+"status = ?", "published").
			Where("NOT EXISTS (SELECT 1 FROM categories WHERE categories.id = "+p+"category_id AND categories.enabled = ?)", false)
	}
}
