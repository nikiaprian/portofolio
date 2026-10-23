package scraper

import (
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// NormalizeWebSearchSource returns a supported web image provider id for scraper/manual search.
func NormalizeWebSearchSource(source string) string {
	s := strings.ToLower(strings.TrimSpace(source))
	switch s {
	case entity.WebImageProviderOpenverse,
		entity.WebImageProviderWikimedia,
		entity.WebImageProviderUnsplash,
		entity.WebImageProviderPixabay,
		entity.WebImageProviderPexels,
		entity.WebImageProviderWordPress:
		return s
	default:
		return "auto"
	}
}
