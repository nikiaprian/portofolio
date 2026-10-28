package entity

import "time"

// tag entity model
type Tag struct {
	ID              uint       `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Name            string     `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Slug            string     `gorm:"size:150;uniqueIndex;not null" json:"slug"`
	Posts           []Post     `gorm:"many2many:post_tags;" json:"posts,omitempty" swaggerignore:"true"`
	IndexingStatus  string     `gorm:"column:indexing_status;size:32;default:not_indexed" json:"indexing_status"`
	IndexedAt       *time.Time `gorm:"column:indexed_at" json:"indexed_at"`
	IndexingMessage string     `gorm:"column:indexing_message;type:text" json:"indexing_message"`
	CreatedAt       time.Time  `json:"created_at" swaggerignore:"true"`
	UpdatedAt       time.Time  `json:"updated_at" swaggerignore:"true"`
}
