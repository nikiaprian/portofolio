package entity

import "time"

// ScraperSetting stores global news scraper options (singleton row).
// Per-category feed URLs live in ScraperCategoryFeed.
type ScraperSetting struct {
	ID           uint `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Enabled      bool `gorm:"column:enabled;default:false" json:"enabled"`
	IntervalMins int  `gorm:"column:interval_mins;default:5" json:"interval_mins"`
	// PauseBetweenCategoriesMins waits this many minutes after each category (in slug order) before the next, within one run (0 = none).
	PauseBetweenCategoriesMins int       `gorm:"column:pause_between_categories_mins;not null;default:0" json:"pause_between_categories_mins"`
	AuthorEmail                string    `gorm:"column:author_email;size:255" json:"author_email"`
	CreatedAt                  time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt                  time.Time `json:"updated_at" swaggerignore:"true"`
}
