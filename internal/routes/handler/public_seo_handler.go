package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
)

// PublicSEOHandler serves public SEO text/XML files via the REST API.
type PublicSEOHandler struct {
	db *gorm.DB
}

// NewPublicSEOHandler constructs PublicSEOHandler.
func NewPublicSEOHandler(db *gorm.DB) *PublicSEOHandler {
	return &PublicSEOHandler{db: db}
}

// RobotsTxt serves robots.txt content from site settings.
func (h *PublicSEOHandler) RobotsTxt(c *gin.Context) {
	seo.ServeRobotsTxt(c, h.db)
}

// AdsTxt serves ads.txt content from site settings.
func (h *PublicSEOHandler) AdsTxt(c *gin.Context) {
	seo.ServeAdsTxt(c, h.db)
}

// SitemapXML serves the sitemap index XML.
func (h *PublicSEOHandler) SitemapXML(c *gin.Context) {
	seo.ServeSitemapIndex(c, h.db, seo.CanonicalBaseURL(c))
}

// SiteScripts serves header/footer script snippets from site settings.
func (h *PublicSEOHandler) SiteScripts(c *gin.Context) {
	seo.ServeSiteScripts(c, h.db)
}
