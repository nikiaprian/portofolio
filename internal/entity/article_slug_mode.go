package entity

import (
	"net/url"
	"strings"
)

const (
	// ArticleSlugModeDirect uses /{post-slug} for published articles.
	ArticleSlugModeDirect = "direct"
	// ArticleSlugModeCategory uses /{category-slug}/{post-slug} (slug kategori yang dipakai artikel, termasuk subkategori).
	ArticleSlugModeCategory = "category"
	// ArticleSlugModeCategoryNested uses /{parent-slug}/{sub-slug}/{post-slug} for posts in a subcategory.
	ArticleSlugModeCategoryNested = "category_nested"
)

// IsValidArticleSlugMode reports whether v is a supported article URL mode (before normalize).
func IsValidArticleSlugMode(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case ArticleSlugModeDirect, ArticleSlugModeCategory, ArticleSlugModeCategoryNested:
		return true
	default:
		return false
	}
}

// NormalizeArticleSlugMode returns a supported article URL mode (default: direct).
func NormalizeArticleSlugMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case ArticleSlugModeCategoryNested:
		return ArticleSlugModeCategoryNested
	case ArticleSlugModeCategory:
		return ArticleSlugModeCategory
	default:
		return ArticleSlugModeDirect
	}
}

// ArticleSlugModeUsesCategoryPrefix reports whether article URLs include a category path segment.
func ArticleSlugModeUsesCategoryPrefix(mode string) bool {
	switch NormalizeArticleSlugMode(mode) {
	case ArticleSlugModeCategory, ArticleSlugModeCategoryNested:
		return true
	default:
		return false
	}
}

// CategoryPathPrefixForArticleURL returns the category path prefix for article URLs (no leading slash).
// category: slug kategori artikel (subkategori → /sub/post); category_nested: parent/child.
func CategoryPathPrefixForArticleURL(mode string, c Category) string {
	if c.ID == 0 {
		return ""
	}
	switch NormalizeArticleSlugMode(mode) {
	case ArticleSlugModeCategoryNested:
		return strings.TrimPrefix(CategoryPublicWebPath(c), "/")
	case ArticleSlugModeCategory:
		return strings.TrimSpace(c.Slug)
	default:
		return ""
	}
}

// PublishedArticleWebPath returns the root-relative canonical path for a published post.
// categoryPathPrefix is from CategoryPathPrefixForArticleURL (e.g. "tech", "tech/mobile").
func PublishedArticleWebPath(articleSlugMode, categoryPathPrefix, postSlug string) string {
	ps := strings.TrimSpace(postSlug)
	if ps == "" {
		return "/"
	}
	escPost := url.PathEscape(ps)
	if ArticleSlugModeUsesCategoryPrefix(articleSlugMode) {
		cp := strings.Trim(strings.TrimSpace(categoryPathPrefix), "/")
		if cp != "" {
			return "/" + cp + "/" + escPost
		}
	}
	return "/" + escPost
}

// PublishedCategoryWebPath returns the root-relative canonical path for a category listing page.
func PublishedCategoryWebPath(categorySlug string) string {
	cs := strings.TrimSpace(categorySlug)
	if cs == "" {
		return "/"
	}
	return "/" + url.PathEscape(cs)
}
