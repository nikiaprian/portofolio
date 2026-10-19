package scraper

import (
	"strings"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// NormalizeWebSearchSource returns a supported web image provider id for scraper/manual search.
func NormalizeWebSearchSource(source string) string {
	s := strings.ToLower(strings.TrimSpace(source))
	switch s {
	case entity.WebImageProviderUnsplash,
		entity.WebImageProviderPexels,
		entity.WebImageProviderGoogle,
		entity.WebImageProviderBing:
		return s
	default:
		return "auto"
	}
}
