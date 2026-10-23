package entity

import "time"

// BulkPostLog records AI bulk-post actions (title suggest + batch generate) for audit.
type BulkPostLog struct {
	ID         uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Level      string    `gorm:"size:16;not null;default:info;index" json:"level"`
	Event      string    `gorm:"size:32;index" json:"event"`
	Message    string    `gorm:"type:text;not null" json:"message"`
	CategoryID *uint     `gorm:"index" json:"category_id,omitempty"`
	PostID       *uint     `gorm:"index" json:"post_id,omitempty"`
	Title        string    `gorm:"size:220" json:"title,omitempty"`
	ProviderUsed string    `gorm:"size:16;index" json:"provider_used,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at" swaggerignore:"true"`
	User         User       `gorm:"foreignKey:UserID;references:ID" json:"-" swaggerignore:"true"`
	Category     *Category  `gorm:"foreignKey:CategoryID;references:ID" json:"-" swaggerignore:"true"`
}

// TableName maps BulkPostLog to bulk_post_logs.
func (BulkPostLog) TableName() string {
	return "bulk_post_logs"
}
