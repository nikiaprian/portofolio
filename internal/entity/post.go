package entity

import "time"

const (
	ImageOriginScrape    = "scrape"
	ImageOriginWebSearch = "web_search"
	ImageOriginTemplate  = "template"
	ImageOriginAI        = "ai"
	ImageOriginManual    = "manual"

	IndexingStatusNotIndexed = "not_indexed"
	IndexingStatusSubmitted  = "submitted"
	IndexingStatusIndexed    = "indexed"
	IndexingStatusFailed     = "failed"
)

// post entity model
type Post struct {
	ID    uint   `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Title string `gorm:"size:200;not null" json:"title"`
	Slug  string `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	// MEDIUMTEXT: HTML from TinyMCE/paste often exceeds MySQL TEXT (64KB).
	Content string  `gorm:"type:mediumtext;not null" json:"content"`
	Image   *string `gorm:"size:500" json:"image,omitempty" swaggerignore:"true"`
	// ImageOrigin records how the cover was obtained (scrape, web_search, template, ai, manual).
	ImageOrigin string `gorm:"column:image_origin;size:20" json:"image_origin,omitempty"`
	// ImageOriginDetail: web_search provider id, scrape source host/URL snippet, etc.
	ImageOriginDetail string  `gorm:"column:image_origin_detail;size:200" json:"image_origin_detail,omitempty"`
	MetaDesc          *string `gorm:"size:500" json:"meta_description,omitempty" swaggerignore:"true"`
	MetaKeywords      *string `gorm:"size:500" json:"meta_keywords,omitempty" swaggerignore:"true"`
	// Source is the canonical article URL for scraper dedupe (TEXT: varchar(500) truncation caused missed duplicates).
	Source *string `gorm:"column:source;type:text" json:"source,omitempty"`
	// No gorm "default:true" here: GORM replaces false with that default on Create (scraped posts could never save include_source=false).
	IncludeSource bool   `gorm:"column:include_source;not null" json:"include_source"`
	Status        string `gorm:"size:20;default:draft" json:"status"`
	// PublishAt is when a scheduled post should go live (UTC). Used with status "scheduled"; cleared when promoted to published.
	PublishAt       *time.Time `json:"publish_at,omitempty"`
	AuthorID        uint       `json:"author_id"`
	Author          User       `gorm:"foreignKey:AuthorID" json:"author"`
	CategoryID      uint       `json:"category_id"`
	Category        Category   `json:"category"`
	Tags            []Tag      `gorm:"many2many:post_tags;" json:"tags"`
	IndexingStatus  string     `gorm:"column:indexing_status;size:30;default:not_indexed" json:"indexing_status,omitempty"`
	IndexedAt       *time.Time `gorm:"column:indexed_at" json:"indexed_at,omitempty"`
	IndexingMessage *string    `gorm:"column:indexing_message;size:500" json:"indexing_message,omitempty"`
	// Populated only by queries that alias COUNT(post_views) AS view; not a real column (see post_views).
	View      int64     `gorm:"->;-:migration" json:"view" swaggerignore:"true"`
	CreatedAt time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt time.Time `json:"updated_at" swaggerignore:"true"`
}
