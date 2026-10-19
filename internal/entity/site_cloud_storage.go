package entity

import "time"

// DefaultSiteCloudStorage returns baseline values for Cloudflare R2 settings.
func DefaultSiteCloudStorage() SiteCloudStorage {
	return SiteCloudStorage{
		R2AccountID:       "",
		R2AccessKeyID:     "",
		R2SecretAccessKey: "",
		R2Bucket:          "media",
		R2PublicBaseURL:   "",
		R2Prefix:          "uploads",
	}
}

// SiteCloudStorage stores singleton cloud storage settings (Cloudflare R2).
type SiteCloudStorage struct {
	ID                uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	R2AccountID       string    `gorm:"column:r2_account_id;size:128" json:"r2_account_id"`
	R2AccessKeyID     string    `gorm:"column:r2_access_key_id;size:128" json:"r2_access_key_id"`
	R2SecretAccessKey string    `gorm:"column:r2_secret_access_key;size:256" json:"r2_secret_access_key"`
	R2Bucket          string    `gorm:"column:r2_bucket;size:120" json:"r2_bucket"`
	R2PublicBaseURL   string    `gorm:"column:r2_public_base_url;size:500" json:"r2_public_base_url"`
	R2Prefix          string    `gorm:"column:r2_prefix;size:255" json:"r2_prefix"`
	CreatedAt         time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt         time.Time `json:"updated_at" swaggerignore:"true"`
}
