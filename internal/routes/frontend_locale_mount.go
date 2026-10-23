package routes

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/config"
)

// RegisterFrontendRoutesWithPrefix mounts frontend/admin routes under an optional path prefix.
// For non-empty prefix, requests are forwarded to a dedicated locale engine so the same handlers can be reused.
func RegisterFrontendRoutesWithPrefix(r *gin.Engine, db *gorm.DB, cfg config.Config, prefix string) {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		RegisterFrontendRoutes(r, db, cfg)
		return
	}
	sub := gin.New()
	sub.Use(gin.Recovery())
	sub.Use(gin.Logger())
	sub.Use(func(c *gin.Context) {
		c.Set("locale", "en")
		c.Set("locale_prefix", prefix)
		c.Next()
	})
	RegisterFrontendRoutes(sub, db, cfg)

	forward := func(c *gin.Context) {
		orig := c.Request.URL.Path
		trimmed := strings.TrimPrefix(orig, prefix)
		if trimmed == "" {
			trimmed = "/"
		}
		c.Set("locale", "en")
		c.Set("locale_prefix", prefix)
		c.Set("external_path", orig)
		c.Request.URL.Path = trimmed
		sub.HandleContext(c)
		c.Abort()
	}
	// Serve locale-prefixed static assets directly (e.g. /en/static/js/app.js).
	r.Static(prefix+"/static", "./public/static")
	// Dispatch all locale-prefixed frontend paths, except API/static/health handled by main engine.
	r.Use(func(c *gin.Context) {
		p := strings.TrimSpace(c.Request.URL.Path)
		if p == "" || !(p == prefix || strings.HasPrefix(p, prefix+"/")) {
			c.Next()
			return
		}
		if p == prefix+"/health" ||
			strings.HasPrefix(p, prefix+"/api/") ||
			strings.HasPrefix(p, prefix+"/static/") {
			c.Next()
			return
		}
		forward(c)
	})
}
