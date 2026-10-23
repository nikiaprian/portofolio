package entity

import (
	"net/url"
	"strings"
)

// CategoryPublicWebPath returns the canonical root-relative path for a category listing page.
// Root categories: /{slug}. Subcategories: /{parent-slug}/{slug} (parent must be loaded on c.Parent).
func CategoryPublicWebPath(c Category) string {
	slug := strings.TrimSpace(c.Slug)
	if slug == "" {
		return "/"
	}
	if c.IsRoot() {
		return "/" + url.PathEscape(slug)
	}
	if c.Parent != nil {
		ps := strings.TrimSpace(c.Parent.Slug)
		if ps != "" {
			return "/" + url.PathEscape(ps) + "/" + url.PathEscape(slug)
		}
	}
	return "/" + url.PathEscape(slug)
}

// CategoryWebPathPrefix is the category path without a leading slash (e.g. "tech" or "tech/mobile").
// Used as the category segment of published article URLs in category slug mode.
func CategoryWebPathPrefix(c Category) string {
	return strings.TrimPrefix(CategoryPublicWebPath(c), "/")
}
