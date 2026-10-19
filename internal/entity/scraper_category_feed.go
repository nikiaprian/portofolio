package entity

import (
	"strings"
	"time"
)

// CanonicalCategorySlug normalizes category_slug for comparisons and deduping.
// MySQL unique indexes on utf8mb4_unicode_ci treat different casings as duplicates;
// maps keyed only by raw slug can miss an existing row and trigger Error 1062.
func CanonicalCategorySlug(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ScraperCategoryFeed stores RSS/Atom feed URLs for one category (one row per category).
type ScraperCategoryFeed struct {
	ID            uint   `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	CategorySlug  string `gorm:"column:category_slug;size:150;uniqueIndex;not null" json:"category_slug"`
	FeedURLs      string `gorm:"column:feed_urls;type:text" json:"feed_urls"`
	Enabled       bool   `gorm:"column:enabled;not null" json:"enabled"`
	IncludeSource bool   `gorm:"column:include_source;not null;default:true" json:"include_source"`
	// ArticleStatus controls status for newly scraped posts in this category.
	// Allowed values: "draft" | "published".
	ArticleStatus string `gorm:"column:article_status;not null;size:20;default:published" json:"article_status"`
	// ImageMode controls how scraper sets post cover images per category.
	// Allowed values:
	// - "scrape": download image from scraped article
	// - "generate": generate a cover image template (bulk cover generator)
	// - "generate_image": AI-generated cover (OpenAI / Gemini; same as admin bulk "generate image")
	// - "web_search": download a relevant public web image by title
	ImageMode string `gorm:"column:image_mode;not null;size:20;default:scrape" json:"image_mode"`
	// WebSearchSource limits web_search to one provider (auto, pexels, unsplash, google, bing).
	WebSearchSource string `gorm:"column:web_search_source;not null;size:20;default:auto" json:"web_search_source"`
	// ImageToContent inserts generated image into post content when ImageMode is generate_image.
	ImageToContent bool `gorm:"column:image_to_content;not null;default:false" json:"image_to_content"`
	// AutoTag when true runs scraper tag inference, in-body tag links, and post tag associations.
	AutoTag bool `gorm:"column:auto_tag;not null;default:true" json:"auto_tag"`
	// AuthorEmail is the CMS user email used as post author for items scraped into this category.
	AuthorEmail string    `gorm:"column:author_email;size:255" json:"author_email"`
	CreatedAt   time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt   time.Time `json:"updated_at" swaggerignore:"true"`
}
