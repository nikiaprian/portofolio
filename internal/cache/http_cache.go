package cache

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type HTTPCacheConfig struct {
	TTL                  time.Duration
	Namespace            string
	CacheAuthorizedAdmin bool
}

type cacheEntry struct {
	Status      int               `json:"status"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        []byte            `json:"body"`
}

type responseRecorder struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *responseRecorder) Write(b []byte) (int, error) {
	_, _ = w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseRecorder) WriteString(s string) (int, error) {
	_, _ = w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func NewRedisHTTPReadCache(client *redis.Client, cfg HTTPCacheConfig) gin.HandlerFunc {
	if client == nil {
		return func(c *gin.Context) { c.Next() }
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 3 * time.Minute
	}
	if strings.TrimSpace(cfg.Namespace) == "" {
		cfg.Namespace = "tirtayasa"
	}

	return func(c *gin.Context) {
		if !isCacheableReadRequest(c, cfg) {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		version := cacheVersion(ctx, client, cfg.Namespace)
		key := buildCacheKey(cfg.Namespace, version, c.Request, cfg)

		if raw, err := client.Get(ctx, key).Bytes(); err == nil {
			var ent cacheEntry
			if json.Unmarshal(raw, &ent) == nil && ent.Status > 0 {
				for k, v := range ent.Headers {
					c.Header(k, v)
				}
				if ent.ContentType != "" {
					c.Header("Content-Type", ent.ContentType)
				}
				c.Header("X-Redis-Cache", "HIT")
				c.Data(ent.Status, ent.ContentType, ent.Body)
				c.Abort()
				return
			}
		}

		rec := &responseRecorder{ResponseWriter: c.Writer}
		c.Writer = rec
		c.Next()

		if c.Writer.Status() != http.StatusOK || c.Writer.Size() <= 0 {
			return
		}
		ent := cacheEntry{
			Status:      c.Writer.Status(),
			ContentType: c.Writer.Header().Get("Content-Type"),
			Headers: map[string]string{
				"Cache-Control": c.Writer.Header().Get("Cache-Control"),
				"ETag":          c.Writer.Header().Get("ETag"),
			},
			Body: rec.body.Bytes(),
		}
		raw, err := json.Marshal(ent)
		if err == nil {
			_ = client.Set(ctx, key, raw, cfg.TTL).Err()
		}
		c.Header("X-Redis-Cache", "MISS")
	}
}

func NewRedisWriteInvalidator(client *redis.Client, namespace string) gin.HandlerFunc {
	if client == nil {
		return func(c *gin.Context) { c.Next() }
	}
	if strings.TrimSpace(namespace) == "" {
		namespace = "tirtayasa"
	}
	return func(c *gin.Context) {
		c.Next()
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if c.Writer.Status() > 0 && c.Writer.Status() < 500 {
				_ = client.Incr(context.Background(), cacheVersionKey(namespace)).Err()
			}
		}
	}
}

func isCacheableReadRequest(c *gin.Context, cfg HTTPCacheConfig) bool {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return false
	}
	if c.Query("no_cache") == "1" {
		return false
	}
	p := c.Request.URL.Path
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	isAdminAPI := strings.HasPrefix(p, "/api/v1/admin")
	if isAdminAPI && authHeader != "" && cfg.CacheAuthorizedAdmin {
		return true
	}
	if authHeader != "" {
		return false
	}
	if isAdminAPI || strings.HasPrefix(p, "/api/v1/auth") {
		return false
	}
	if strings.HasPrefix(p, "/static/") {
		return false
	}
	// Site-controlled text/XML; must not serve stale bodies from Redis after admin edits.
	if p == "/robots.txt" || p == "/ads.txt" || p == "/sitemap.xml" {
		return false
	}
	return true
}

func cacheVersion(ctx context.Context, client *redis.Client, ns string) string {
	v, err := client.Get(ctx, cacheVersionKey(ns)).Result()
	if err == nil && v != "" {
		return v
	}
	return "0"
}

func cacheVersionKey(ns string) string {
	return ns + ":http_cache:version"
}

func buildCacheKey(namespace, version string, r *http.Request, cfg HTTPCacheConfig) string {
	authScope := "public"
	if cfg.CacheAuthorizedAdmin {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if auth != "" {
			s := sha1.Sum([]byte(auth))
			authScope = "auth:" + hex.EncodeToString(s[:])
		}
	}
	base := strings.Join([]string{
		r.Method,
		r.Host,
		r.URL.Path,
		r.URL.RawQuery,
		r.Header.Get("Accept"),
		r.Header.Get("Accept-Language"),
		authScope,
	}, "|")
	sum := sha1.Sum([]byte(base))
	return namespace + ":http_cache:" + version + ":" + hex.EncodeToString(sum[:])
}
