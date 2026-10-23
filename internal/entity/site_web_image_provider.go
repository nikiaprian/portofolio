package entity

import "time"

const (
	WebImageProviderOpenverse = "openverse"
	WebImageProviderWikimedia = "wikimedia"
	WebImageProviderUnsplash  = "unsplash"
	WebImageProviderPixabay   = "pixabay"
	WebImageProviderPexels    = "pexels"
	WebImageProviderWordPress = "wordpress"
)

// WebImageProviderAutoOrder is the provider try order when web_search source is auto
// (scraper, manual post create, bulk post).
var WebImageProviderAutoOrder = []string{
	WebImageProviderPexels,
	WebImageProviderUnsplash,
	WebImageProviderPixabay,
	WebImageProviderOpenverse,
	WebImageProviderWikimedia,
	WebImageProviderWordPress,
}

// SiteWebImageProvider stores credentials for public image search providers.
// CredentialsJSON is provider-specific JSON so new providers can add fields without schema churn.
type SiteWebImageProvider struct {
	ID              uint      `gorm:"primaryKey" json:"id" swaggerignore:"true"`
	Provider        string    `gorm:"column:provider;size:40;uniqueIndex;not null" json:"provider"`
	Enabled         bool      `gorm:"column:enabled;not null;default:false" json:"enabled"`
	CredentialsJSON string    `gorm:"column:credentials_json;type:text" json:"credentials_json"`
	CreatedAt       time.Time `json:"created_at" swaggerignore:"true"`
	UpdatedAt       time.Time `json:"updated_at" swaggerignore:"true"`
}

func DefaultSiteWebImageProviders() []SiteWebImageProvider {
	out := make([]SiteWebImageProvider, 0, len(WebImageProviderAutoOrder))
	for _, provider := range WebImageProviderAutoOrder {
		out = append(out, SiteWebImageProvider{Provider: provider, Enabled: false, CredentialsJSON: "{}"})
	}
	return out
}
