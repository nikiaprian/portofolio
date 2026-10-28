package gsc

import (
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
)

// PublicSiteOrigin returns the public site origin for indexing URLs.
// Prefers PUBLIC_SITE_URL, then the incoming request host, then localhost.
func PublicSiteOrigin(c *gin.Context) string {
	if o := envOrigin("PUBLIC_SITE_URL"); o != "" {
		return o
	}
	if c != nil && c.Request != nil {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		if host := strings.TrimSpace(c.Request.Host); host != "" {
			return scheme + "://" + host
		}
	}
	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if port == "" {
		port = "8080"
	}
	return "http://localhost:" + port
}

func envOrigin(key string) string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func joinOrigin(baseOrigin, rel string) string {
	baseOrigin = strings.TrimSpace(baseOrigin)
	if baseOrigin == "" {
		baseOrigin = "http://localhost:8080"
	}
	if rel == "" || rel == "/" {
		return strings.TrimRight(baseOrigin, "/") + "/"
	}
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	return strings.TrimRight(baseOrigin, "/") + rel
}

// BuildPublicPostURL constructs the canonical public URL for a post.
func BuildPublicPostURL(db *gorm.DB, p *entity.Post, baseOrigin string) string {
	if p == nil || strings.TrimSpace(p.Slug) == "" {
		return joinOrigin(baseOrigin, "/")
	}
	mode := repositories.ArticleSlugModeFromDB(db)
	cat := loadedPostCategory(db, p)
	prefix := entity.CategoryPathPrefixForArticleURL(mode, cat)
	return joinOrigin(baseOrigin, entity.PublishedArticleWebPath(mode, prefix, p.Slug))
}

func loadedPostCategory(db *gorm.DB, p *entity.Post) entity.Category {
	if p == nil {
		return entity.Category{}
	}
	cat := p.Category
	if cat.ID == 0 && p.CategoryID != 0 && db != nil {
		_ = db.Preload("Parent").First(&cat, p.CategoryID).Error
	}
	if db != nil && !cat.IsRoot() && cat.ParentID != nil && (cat.Parent == nil || cat.Parent.ID == 0) {
		var parent entity.Category
		if err := db.Select("id", "name", "slug").First(&parent, *cat.ParentID).Error; err == nil {
			cat.Parent = &parent
		}
	}
	return cat
}

// BuildPublicCategoryURL constructs the canonical public URL for a category listing.
func BuildPublicCategoryURL(db *gorm.DB, cat *entity.Category, baseOrigin string) string {
	if cat == nil || strings.TrimSpace(cat.Slug) == "" {
		return joinOrigin(baseOrigin, "/")
	}
	current := *cat
	if db != nil && !current.IsRoot() && current.ParentID != nil && (current.Parent == nil || current.Parent.ID == 0) {
		var parent entity.Category
		if err := db.Select("id", "name", "slug").First(&parent, *current.ParentID).Error; err == nil {
			current.Parent = &parent
		}
	}
	return joinOrigin(baseOrigin, entity.CategoryPublicWebPath(current))
}

// BuildPublicTagURL constructs the canonical public URL for a tag listing.
func BuildPublicTagURL(_ *gorm.DB, tag *entity.Tag, baseOrigin string) string {
	if tag == nil || strings.TrimSpace(tag.Slug) == "" {
		return joinOrigin(baseOrigin, "/")
	}
	return joinOrigin(baseOrigin, "/tag/"+url.PathEscape(strings.TrimSpace(tag.Slug)))
}

// BuildPublicPageURL constructs the canonical public URL for a standalone page.
func BuildPublicPageURL(_ *gorm.DB, p *entity.Page, baseOrigin string) string {
	if p == nil || strings.TrimSpace(p.Slug) == "" {
		return joinOrigin(baseOrigin, "/")
	}
	return joinOrigin(baseOrigin, "/"+url.PathEscape(strings.TrimSpace(p.Slug)))
}
