package entity

import "time"

// SiteLayout stores reusable client template fragments per named layout.
type SiteLayout struct {
	ID            uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	LayoutName    string    `gorm:"column:layout_name;size:255;not null;uniqueIndex" json:"layout_name"`
	Header        string    `gorm:"type:text" json:"header"`
	Footer        string    `gorm:"type:text" json:"footer"`
	HomeBody      string    `gorm:"column:home_body;type:text" json:"home_body"`
	ArticleBody   string    `gorm:"column:article_body;type:text" json:"article_body"`
	Sidebar       string    `gorm:"type:text" json:"sidebar"`
	RSS           string    `gorm:"column:rss;type:text" json:"rss"`
	AuthorProfile string    `gorm:"column:author_profile;type:text" json:"author_profile"`
	Tag           string    `gorm:"column:tag;type:text" json:"tag"`
	Kategori      string    `gorm:"column:kategori;type:text" json:"kategori"`
	SearchPage    string    `gorm:"column:search_page;type:text" json:"search_page"`
	Misc          string    `gorm:"column:misc;type:text" json:"misc"`
	LatestPrompt  string    `gorm:"column:latest_prompt;type:text" json:"latest_prompt"`
	CurrentPrompt string    `gorm:"column:current_prompt;type:text" json:"current_prompt"`
	ImageRef      string    `gorm:"column:image_ref;type:text" json:"image_ref"`
	CSSData       string    `gorm:"column:css_data;type:text" json:"css_data"`
	JSData        string    `gorm:"column:js_data;type:text" json:"js_data"`
	IsUsed        bool      `gorm:"column:is_used;not null;default:false" json:"is_used"`
	CreatedAt     time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt     time.Time `json:"updated_at" swaggerignore:"true"`
}

func (SiteLayout) TableName() string { return "site_layouts" }
