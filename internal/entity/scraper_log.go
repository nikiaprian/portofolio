package entity

import "time"

// ScraperLog stores one line/event from the news scraper worker (rolling retention via ADMIN_LOG_RETENTION_HOURS with other admin logs).
type ScraperLog struct {
	ID           uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Level        string    `gorm:"size:16;not null;default:info;index" json:"level"`
	Message      string    `gorm:"type:text;not null" json:"message"`
	CategorySlug string    `gorm:"size:255;index" json:"category_slug"`
	FeedURL      string    `gorm:"size:2000" json:"feed_url"`
	ProviderUsed string    `gorm:"size:16;index" json:"provider_used,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at" swaggerignore:"true"`
}

func (ScraperLog) TableName() string {
	return "scraper_logs"
}
