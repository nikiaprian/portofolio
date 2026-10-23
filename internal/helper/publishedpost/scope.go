package publishedpost

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ScopeVisiblePublished limits rows to published posts.
// The now parameter is retained for call-site compatibility; it is not used for filtering.
// Use tablePrefix "" for the posts table without alias, or "posts" when joining under that alias.
func ScopeVisiblePublished(tablePrefix string, now time.Time) func(*gorm.DB) *gorm.DB {
	_ = now
	return func(db *gorm.DB) *gorm.DB {
		p := strings.TrimSpace(tablePrefix)
		if p != "" && !strings.HasSuffix(p, ".") {
			p += "."
		}
		return db.Where(p+"status = ?", "published")
	}
}
