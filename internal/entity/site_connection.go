package entity

import "time"

// SiteConnection stores webmaster verification tokens (singleton row).
type SiteConnection struct {
	ID     uint   `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Ahrefs string `gorm:"column:ahrefs;size:512" json:"ahrefs"`
	Google string `gorm:"column:google;size:512" json:"google"`
	Bing   string `gorm:"column:bing;size:512" json:"bing"`
	Yandex string `gorm:"column:yandex;size:512" json:"yandex"`

	AutoPostEnabled  bool   `gorm:"column:auto_post_enabled;default:false" json:"auto_post_enabled"`
	AutoPostTemplate string `gorm:"column:auto_post_template;type:text" json:"auto_post_template"`

	FacebookEnabled      bool   `gorm:"column:facebook_enabled;default:false" json:"facebook_enabled"`
	FacebookClientID     string `gorm:"column:facebook_client_id;size:255" json:"facebook_client_id"`
	FacebookClientSecret string `gorm:"column:facebook_client_secret;size:255" json:"facebook_client_secret"`
	FacebookTokenURL     string `gorm:"column:facebook_token_url;size:500" json:"facebook_token_url"`
	FacebookPublishURL   string `gorm:"column:facebook_publish_url;size:500" json:"facebook_publish_url"`
	FacebookScope        string `gorm:"column:facebook_scope;size:500" json:"facebook_scope"`
	FacebookAccountID    string `gorm:"column:facebook_account_id;size:255" json:"facebook_account_id"`

	XEnabled      bool   `gorm:"column:x_enabled;default:false" json:"x_enabled"`
	XClientID     string `gorm:"column:x_client_id;size:255" json:"x_client_id"`
	XClientSecret string `gorm:"column:x_client_secret;size:255" json:"x_client_secret"`
	XTokenURL     string `gorm:"column:x_token_url;size:500" json:"x_token_url"`
	XPublishURL   string `gorm:"column:x_publish_url;size:500" json:"x_publish_url"`
	XScope        string `gorm:"column:x_scope;size:500" json:"x_scope"`
	XAccountID    string `gorm:"column:x_account_id;size:255" json:"x_account_id"`

	TiktokEnabled      bool   `gorm:"column:tiktok_enabled;default:false" json:"tiktok_enabled"`
	TiktokClientID     string `gorm:"column:tiktok_client_id;size:255" json:"tiktok_client_id"`
	TiktokClientSecret string `gorm:"column:tiktok_client_secret;size:255" json:"tiktok_client_secret"`
	TiktokTokenURL     string `gorm:"column:tiktok_token_url;size:500" json:"tiktok_token_url"`
	TiktokPublishURL   string `gorm:"column:tiktok_publish_url;size:500" json:"tiktok_publish_url"`
	TiktokScope        string `gorm:"column:tiktok_scope;size:500" json:"tiktok_scope"`
	TiktokAccountID    string `gorm:"column:tiktok_account_id;size:255" json:"tiktok_account_id"`

	InstagramEnabled      bool   `gorm:"column:instagram_enabled;default:false" json:"instagram_enabled"`
	InstagramClientID     string `gorm:"column:instagram_client_id;size:255" json:"instagram_client_id"`
	InstagramClientSecret string `gorm:"column:instagram_client_secret;size:255" json:"instagram_client_secret"`
	InstagramTokenURL     string `gorm:"column:instagram_token_url;size:500" json:"instagram_token_url"`
	InstagramPublishURL   string `gorm:"column:instagram_publish_url;size:500" json:"instagram_publish_url"`
	InstagramScope        string `gorm:"column:instagram_scope;size:500" json:"instagram_scope"`
	InstagramAccountID    string `gorm:"column:instagram_account_id;size:255" json:"instagram_account_id"`

	CreatedAt time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt time.Time `json:"updated_at" swaggerignore:"true"`
}
