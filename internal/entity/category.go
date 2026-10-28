package entity

import "time"

// category entity model
type Category struct {
	ID   uint   `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Sort int    `gorm:"not null;default:1;index" json:"sort"`
	Name string `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Slug string `gorm:"size:150;uniqueIndex;not null" json:"slug"`
	// ParentID is nil for root categories. Subcategories point to a root category only (max depth 1).
	ParentID *uint      `gorm:"index" json:"parent_id,omitempty"`
	Parent   *Category  `gorm:"foreignKey:ParentID" json:"parent,omitempty" swaggerignore:"true"`
	Children []Category `gorm:"foreignKey:ParentID" json:"children,omitempty" swaggerignore:"true"`
	// Title is the short browser/OG title for the category listing (e.g. "Kumpulan artikel kategori X").
	Title string `gorm:"size:255" json:"title"`
	// Keyword is stored for <meta name="keywords"> on the public category page.
	Keyword string `gorm:"size:500" json:"keyword"`
	// Description is the long meta description for the public category page.
	Description     string     `gorm:"size:500" json:"description"`
	Posts           []Post     `json:"posts,omitempty" swaggerignore:"true"`
	IndexingStatus  string     `gorm:"column:indexing_status;size:32;default:not_indexed" json:"indexing_status"`
	IndexedAt       *time.Time `gorm:"column:indexed_at" json:"indexed_at"`
	IndexingMessage string     `gorm:"column:indexing_message;type:text" json:"indexing_message"`
	CreatedAt       time.Time  `json:"created_at" swaggerignore:"true"`
	UpdatedAt       time.Time  `json:"updated_at" swaggerignore:"true"`
}

// IsRoot reports whether the category has no parent (legacy rows and top-level categories).
func (c Category) IsRoot() bool {
	return c.ParentID == nil || *c.ParentID == 0
}
