package entity

import "time"

// page entity model
type Page struct {
	ID              uint       `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Title           string     `gorm:"size:200;not null" json:"title"`
	Slug            string     `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Content         string     `gorm:"type:text;not null" json:"content"`
	Status          string     `gorm:"size:20;default:draft" json:"status"`
	IndexingStatus  string     `gorm:"column:indexing_status;size:32;default:not_indexed" json:"indexing_status"`
	IndexedAt       *time.Time `gorm:"column:indexed_at" json:"indexed_at"`
	IndexingMessage string     `gorm:"column:indexing_message;type:text" json:"indexing_message"`
	CreatedAt       time.Time  `json:"created_at" swaggerignore:"true"`
	UpdatedAt       time.Time  `json:"updated_at" swaggerignore:"true"`
}
