package seo

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/helper/scraper"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

type sitemapIndex struct {
	XMLName  xml.Name            `xml:"sitemapindex"`
	Xmlns    string              `xml:"xmlns,attr"`
	Sitemaps []sitemapIndexEntry `xml:"sitemap"`
}

type sitemapIndexEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// envOrigin returns scheme://host from an env var (path/query stripped).
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

func forwardedProto(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return "http"
	}
	if p := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); p != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(p, ",")[0]))
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func forwardedHost(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if h := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); h != "" {
		if idx := strings.Index(h, ","); idx >= 0 {
			h = strings.TrimSpace(h[:idx])
		}
		return h
	}
	return c.Request.Host
}

// RequestOrigin returns scheme://host for frontend SSR/XML routes.
// Honors X-Forwarded-Host and X-Forwarded-Proto when a frontend proxy forwards the public site origin.
func RequestOrigin(c *gin.Context) string {
	return forwardedProto(c) + "://" + forwardedHost(c)
}

// DefaultLocalOrigin is used when no request context is available (e.g. background jobs).
func DefaultLocalOrigin() string {
	if o := envOrigin("PUBLIC_SITE_URL"); o != "" {
		return o
	}
	if o := envOrigin("SEO_BASE_URL"); o != "" {
		return o
	}
	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if port == "" {
		port = "8080"
	}
	return "http://localhost:" + port
}

// PublicSiteOrigin returns the public site origin for admin links and social autopost.
// Prefers PUBLIC_SITE_URL, then the incoming request host, then DefaultLocalOrigin.
func PublicSiteOrigin(c *gin.Context) string {
	if o := envOrigin("PUBLIC_SITE_URL"); o != "" {
		return o
	}
	if c != nil {
		return RequestOrigin(c)
	}
	return DefaultLocalOrigin()
}

// MediaOrigin returns the origin that actually serves /static/uploads and /img.
// Uses the real request Host (not X-Forwarded-Host) so proxied RSS/sitemap feeds keep backend asset URLs.
func MediaOrigin(c *gin.Context) string {
	if c == nil || c.Request == nil {
		port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
		if port == "" {
			port = "8080"
		}
		return "http://localhost:" + port
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if host := strings.TrimSpace(c.Request.Host); host != "" {
		return scheme + "://" + host
	}
	port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
	if port == "" {
		port = "8080"
	}
	return "http://localhost:" + port
}

// CanonicalBaseURL returns the public origin for public API SEO endpoints (/api/v1/sitemap.xml, etc.).
// Prefers SEO_BASE_URL, then PUBLIC_SITE_URL, then the incoming request host.
func CanonicalBaseURL(c *gin.Context) string {
	if o := envOrigin("SEO_BASE_URL"); o != "" {
		return o
	}
	if o := envOrigin("PUBLIC_SITE_URL"); o != "" {
		return o
	}
	if c != nil {
		return RequestOrigin(c)
	}
	return DefaultLocalOrigin()
}

// ServeRobotsTxt writes robots.txt from site_settings.
func ServeRobotsTxt(c *gin.Context, db *gorm.DB) {
	body := strings.TrimSpace(repositories.SiteSettingOrDefault(db).RobotsTxt)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(body+"\n"))
}

// ServeAdsTxt writes ads.txt from site_settings.
func ServeAdsTxt(c *gin.Context, db *gorm.DB) {
	body := strings.TrimSpace(repositories.SiteSettingOrDefault(db).AdsTxt)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(body+"\n"))
}

// ServeSiteScripts writes public script/ad snippets from site_settings.
func ServeSiteScripts(c *gin.Context, db *gorm.DB) {
	s := repositories.SiteSettingOrDefault(db)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"header_script":         strings.TrimSpace(s.HeaderScript),
		"footer_script":         strings.TrimSpace(s.FooterScript),
		"ads_top":               strings.TrimSpace(s.AdsTop),
		"ads_center":            strings.TrimSpace(s.AdsCenter),
		"ads_bottom":            strings.TrimSpace(s.AdsBottom),
		"ads_lazy_load_enabled": s.AdsLazyLoadEnabled,
	})
}

// ServeSitemapIndex writes the sitemap index XML.
func ServeSitemapIndex(c *gin.Context, db *gorm.DB, baseURL string) {
	rows, err := repositories.NewCategoryRepository(db).SitemapRows()
	if err != nil {
		c.String(http.StatusInternalServerError, "sitemap error: %v", err)
		return
	}

	items := make([]sitemapIndexEntry, 0, len(rows)+2)
	items = append(items, sitemapIndexEntry{
		Loc: baseURL + "/rss/sitemap-news.xml",
	})
	items = append(items, sitemapIndexEntry{
		Loc: baseURL + "/rss/latest-posts.xml",
	})
	for _, row := range rows {
		loc := baseURL + "/rss/" + row.Slug + ".xml"
		lastMod := ""
		if !row.UpdatedAt.IsZero() {
			lastMod = row.UpdatedAt.Format(time.RFC3339)
		}
		items = append(items, sitemapIndexEntry{
			Loc:     loc,
			LastMod: lastMod,
		})
	}

	c.Header("Cache-Control", "no-store")
	writeXMLResponse(c, sitemapIndex{
		Xmlns:    "http://www.sitemaps.org/schemas/sitemap/0.9",
		Sitemaps: items,
	})
}

func writeXMLResponse(c *gin.Context, payload any) {
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte(xml.Header))
	_, _ = c.Writer.Write([]byte(scraputil.MarkerComment() + "\n"))
	enc := xml.NewEncoder(c.Writer)
	enc.Indent("", "  ")
	_ = enc.Encode(payload)
}
