package middleware

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultPublicSearchMaxLen      = 100
	defaultPublicSearchRateLimit   = 30
	defaultPublicSearchRateWindow  = time.Minute
	publicSearchTooManyRequestsMsg = "Terlalu banyak pencarian. Coba lagi nanti."
)

// PublicSearchMaxQueryLen is the maximum length of a public ?q= search term.
func PublicSearchMaxQueryLen() int {
	if v := envInt("PUBLIC_SEARCH_MAX_LEN", defaultPublicSearchMaxLen); v > 0 {
		return v
	}
	return defaultPublicSearchMaxLen
}

// NormalizePublicSearchQuery trims and caps q; ok is false when empty after normalize.
func NormalizePublicSearchQuery(raw string) (q string, ok bool) {
	q = strings.TrimSpace(raw)
	if q == "" {
		return "", false
	}
	max := PublicSearchMaxQueryLen()
	if len(q) > max {
		q = q[:max]
	}
	return q, true
}

// PublicSearchLimit applies rate limits only when the request includes a non-empty ?q= term.
func PublicSearchLimit() gin.HandlerFunc {
	limit := envInt("PUBLIC_SEARCH_RATE_LIMIT", defaultPublicSearchRateLimit)
	if limit < 1 {
		limit = defaultPublicSearchRateLimit
	}
	windowSec := envInt("PUBLIC_SEARCH_RATE_WINDOW_SEC", int(defaultPublicSearchRateWindow/time.Second))
	if windowSec < 1 {
		windowSec = int(defaultPublicSearchRateWindow / time.Second)
	}
	window := time.Duration(windowSec) * time.Second
	store := newSearchRateStore(window)

	return func(c *gin.Context) {
		q, ok := NormalizePublicSearchQuery(c.Query("q"))
		if !ok {
			c.Next()
			return
		}
		c.Set("public_search_q", q)

		ip := clientIP(c)
		if !store.allow(ip, limit) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusTooManyRequests, gin.H{"error": publicSearchTooManyRequestsMsg})
			} else {
				c.Data(http.StatusTooManyRequests, "text/html; charset=utf-8", []byte(
					"<!DOCTYPE html><html lang=\"id\"><head><meta charset=\"utf-8\"><title>Terlalu banyak pencarian</title></head><body><p>"+
						publicSearchTooManyRequestsMsg+
						"</p><p><a href=\"/\">Kembali ke beranda</a></p></body></html>",
				))
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

func clientIP(c *gin.Context) string {
	if xff := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); xff != "" {
		if i := strings.Index(xff, ","); i >= 0 {
			xff = strings.TrimSpace(xff[:i])
		}
		if xff != "" {
			return xff
		}
	}
	if xri := strings.TrimSpace(c.GetHeader("X-Real-IP")); xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(c.ClientIP()))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(c.ClientIP())
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

type searchRateStore struct {
	window time.Duration
	mu     sync.Mutex
	byIP   map[string]*searchRateBucket
}

type searchRateBucket struct {
	count   int
	resetAt time.Time
}

func newSearchRateStore(window time.Duration) *searchRateStore {
	return &searchRateStore{
		window: window,
		byIP:   make(map[string]*searchRateBucket),
	}
}

func (s *searchRateStore) allow(ip string, limit int) bool {
	if ip == "" {
		ip = "unknown"
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for k, b := range s.byIP {
		if now.After(b.resetAt) {
			delete(s.byIP, k)
		}
	}

	b, ok := s.byIP[ip]
	if !ok || now.After(b.resetAt) {
		s.byIP[ip] = &searchRateBucket{count: 1, resetAt: now.Add(s.window)}
		return true
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}
