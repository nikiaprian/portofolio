package entity

import "time"

// tag entity model
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Name      string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Slug      string    `gorm:"size:150;uniqueIndex;not null" json:"slug"`
	Posts     []Post    `gorm:"many2many:post_tags;" json:"posts,omitempty" swaggerignore:"true"`
	CreatedAt time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt time.Time `json:"updated_at" swaggerignore:"true"`
}
