package publicshelf

import (
	"strings"

	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// Needles are the public-service topics Tirtayasa publishes on the homepage,
// in listings, and in sitemaps. Other published posts stay reachable by URL
// but are left out of those surfaces.
var Needles = []string{
	"bansos",
	"dtsen",
	"dtks",
	"pkh",
	"pip",
	"bsu",
	"blt",
	"bpum",
	"prakerja",
	"indonesia pintar",
	"kip ",
	"kip-",
	"-kip",
	"bpjs",
	"jkn",
	"jht",
	"jkk",
	"jkm",
	"puskesmas",
	"posyandu",
	"skck",
	"nisn",
	"ppdb",
	"snbp",
	"snbt",
	"utbk",
	"ikd",
	"dukcapil",
	"ktp",
	"kartu keluarga",
	"sktm",
	"npwp",
	"pajak",
	"samsat",
	"paspor",
	"imigrasi",
	"cpns",
	"pppk",
	"korlantas",
	"perjanjian kerja",
	"perjanjian-kerja",
	"wawancara",
}

// Active reports whether the public site should stay on that shelf.
func Active(layout string) bool {
	return entity.NormalizeHomeLayout(layout) == entity.HomeLayoutTirtayasa
}

// Matches reports whether a post title or slug belongs on the shelf.
func Matches(title, slug string) bool {
	blob := strings.ToLower(strings.TrimSpace(title) + " " + strings.TrimSpace(slug))
	for _, needle := range Needles {
		if strings.Contains(blob, needle) {
			return true
		}
	}
	return false
}

// Scope limits a posts query to the shelf. prefix is empty or a table alias.
func Scope(prefix string) func(*gorm.DB) *gorm.DB {
	p := strings.TrimSpace(prefix)
	if p != "" && !strings.HasSuffix(p, ".") {
		p += "."
	}
	parts := make([]string, 0, len(Needles)*2)
	args := make([]any, 0, len(Needles)*2)
	for _, needle := range Needles {
		like := "%" + needle + "%"
		parts = append(parts, "LOWER("+p+"title) LIKE ?", "LOWER("+p+"slug) LIKE ?")
		args = append(args, like, like)
	}
	clause := "(" + strings.Join(parts, " OR ") + ")"
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(clause, args...)
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
