package postutil

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

var (
	htmlTagRE = regexp.MustCompile("<[^>]*>")
	spaceRE   = regexp.MustCompile(`\s+`)
	imgSrcRE  = regexp.MustCompile(`(?is)<img[^>]+src\s*=\s*["']([^"']+)["']`)
)

// Apply sets MetaDescription and MetaKeywords from post fields.
// tagNames should be tag display names in order (may be empty). categoryName is optional fallback for keywords.
func Apply(p *entity.Post, tagNames []string, categoryName string) {
	if p == nil {
		return
	}
	title := strings.TrimSpace(p.Title)
	if title == "" {
		return
	}

	mt := truncateRunes(title, 255)

	plain := plainFromHTML(p.Content)
	desc := metaDescriptionFromPlain(plain, 100, 160)
	if desc == "" {
		desc = mt
	}
	p.MetaDesc = strPtr(desc)

	kw := keywordsFromTags(tagNames)
	if kw == "" {
		kw = strings.TrimSpace(categoryName)
	}
	if kw != "" {
		p.MetaKeywords = strPtr(truncateRunes(kw, 500))
	} else {
		p.MetaKeywords = nil
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	t := s
	return &t
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}

// metaDescriptionFromPlain builds SEO description: panjang ideal min–max rune;
// jika ada titik (.) dalam jendela, potong di titik terakhir yang memenuhi minimal minLen.
func metaDescriptionFromPlain(plain string, minLen, maxLen int) string {
	s := strings.TrimSpace(plain)
	if s == "" {
		return ""
	}
	if minLen < 0 {
		minLen = 0
	}
	if maxLen < minLen {
		maxLen = minLen
	}
	r := []rune(s)
	n := len(r)
	if n == 0 {
		return ""
	}
	if n <= maxLen {
		lastDot := -1
		for i := 0; i < n; i++ {
			if r[i] == '.' {
				lastDot = i
			}
		}
		if lastDot >= 0 && lastDot+1 >= minLen {
			return string(r[:lastDot+1])
		}
		return s
	}
	// n > maxLen: titik terakhir pada indeks < maxLen dengan panjang (indeks+1) >= minLen
	bestEnd := -1
	for i := 0; i < maxLen; i++ {
		if r[i] == '.' && i+1 >= minLen {
			bestEnd = i
		}
	}
	if bestEnd >= 0 {
		return string(r[:bestEnd+1])
	}
	return string(r[:maxLen])
}

func plainFromHTML(raw string) string {
	s := htmlTagRE.ReplaceAllString(raw, " ")
	s = html.UnescapeString(s)
	s = strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
	return s
}

// PlainExcerpt returns whitespace-normalized plain text from HTML. When maxRunes > 0, the result is truncated to that many runes.
func PlainExcerpt(html string, maxRunes int) string {
	s := plainFromHTML(html)
	if maxRunes <= 0 {
		return s
	}
	return truncateRunes(s, maxRunes)
}

// FirstImageURL returns the first <img src="..."> from HTML, or empty string.
func FirstImageURL(htmlFrag string) string {
	if m := imgSrcRE.FindStringSubmatch(htmlFrag); len(m) > 1 {
		return strings.TrimSpace(html.UnescapeString(m[1]))
	}
	return ""
}

func keywordsFromTags(names []string) string {
	parts := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			parts = append(parts, n)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

// BuildPublicPostURL constructs full canonical URL for a post (e.g., https://domain.com/category/slug).
func BuildPublicPostURL(db *gorm.DB, p *entity.Post, baseOrigin string) string {
	baseOrigin = strings.TrimSpace(baseOrigin)
	if baseOrigin == "" {
		baseOrigin = "http://localhost:8080"
	}
	if db == nil || p == nil {
		return baseOrigin
	}
	var setting entity.SiteSetting
	_ = db.First(&setting).Error
	mode := setting.ArticleSlugMode
	catPath := ""
	if p.CategoryID > 0 {
		if p.Category.ID > 0 {
			catPath = entity.CategoryWebPathPrefix(p.Category)
		} else {
			var cat entity.Category
			if db.Preload("Parent").First(&cat, p.CategoryID).Error == nil {
				catPath = entity.CategoryWebPathPrefix(cat)
			}
		}
	}
	rel := entity.PublicArticleWebPath(mode, catPath, p.Slug)
	return strings.TrimRight(baseOrigin, "/") + rel
}

// BuildPublicCategoryURL constructs full canonical URL for a category.
func BuildPublicCategoryURL(db *gorm.DB, cat *entity.Category, baseOrigin string) string {
	baseOrigin = strings.TrimSpace(baseOrigin)
	if baseOrigin == "" {
		baseOrigin = "http://localhost:8080"
	}
	if cat == nil {
		return baseOrigin
	}
	rel := entity.CategoryPublicWebPath(*cat)
	return strings.TrimRight(baseOrigin, "/") + rel
}

// BuildPublicTagURL constructs full canonical URL for a tag.
func BuildPublicTagURL(db *gorm.DB, tag *entity.Tag, baseOrigin string) string {
	baseOrigin = strings.TrimSpace(baseOrigin)
	if baseOrigin == "" {
		baseOrigin = "http://localhost:8080"
	}
	if tag == nil {
		return baseOrigin
	}
	rel := "/tag/" + strings.TrimPrefix(strings.TrimSpace(tag.Slug), "/")
	return strings.TrimRight(baseOrigin, "/") + rel
}

// BuildPublicPageURL constructs full canonical URL for a standalone page.
func BuildPublicPageURL(db *gorm.DB, p *entity.Page, baseOrigin string) string {
	baseOrigin = strings.TrimSpace(baseOrigin)
	if baseOrigin == "" {
		baseOrigin = "http://localhost:8080"
	}
	if p == nil {
		return baseOrigin
	}
	rel := "/" + strings.TrimPrefix(strings.TrimSpace(p.Slug), "/")
	return strings.TrimRight(baseOrigin, "/") + rel
}

