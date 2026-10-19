package entity

import "time"

const (
	WebImageProviderUnsplash = "unsplash"
	WebImageProviderPexels   = "pexels"
	WebImageProviderGoogle   = "google"
	WebImageProviderBing     = "bing"
	WebImageProviderBingNet  = "bing_net"
	WebImageProviderSerpAPI  = "serpapi"
)

// WebImageProviderConfigOrder lists providers stored in site_web_image_providers.
var WebImageProviderConfigOrder = []string{
	WebImageProviderPexels,
	WebImageProviderUnsplash,
	WebImageProviderSerpAPI,
}

// WebImageProviderAutoOrder is the provider try order when web_search source is auto
// (scraper, manual post create, bulk post).
var WebImageProviderAutoOrder = []string{
	WebImageProviderPexels,
	WebImageProviderUnsplash,
	WebImageProviderBingNet,
	WebImageProviderGoogle,
	WebImageProviderBing,
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
	out := make([]SiteWebImageProvider, 0, len(WebImageProviderConfigOrder))
	for _, provider := range WebImageProviderConfigOrder {
		out = append(out, SiteWebImageProvider{Provider: provider, Enabled: false, CredentialsJSON: "{}"})
	}
	return out
}
