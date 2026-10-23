package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNormalizePublicSearchQuery(t *testing.T) {
	t.Setenv("PUBLIC_SEARCH_MAX_LEN", "10")

	q, ok := NormalizePublicSearchQuery("  hello  ")
	if !ok || q != "hello" {
		t.Fatalf("got %q ok=%v", q, ok)
	}

	q, ok = NormalizePublicSearchQuery("abcdefghijklmnop")
	if !ok || q != "abcdefghij" {
		t.Fatalf("got %q ok=%v", q, ok)
	}

	_, ok = NormalizePublicSearchQuery("   ")
	if ok {
		t.Fatal("expected empty")
	}
}

func TestPublicSearchLimit_rateCaps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PUBLIC_SEARCH_RATE_LIMIT", "2")
	t.Setenv("PUBLIC_SEARCH_RATE_WINDOW_SEC", "60")

	r := gin.New()
	r.GET("/search-test", PublicSearchLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/search-test?q=foo", nil)
		req.RemoteAddr = "203.0.113.1:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search-test?q=foo", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestPublicSearchLimit_skipsWithoutQ(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PUBLIC_SEARCH_RATE_LIMIT", "1")

	r := gin.New()
	r.GET("/search-test", PublicSearchLimit(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/search-test", nil)
		req.RemoteAddr = "203.0.113.2:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i+1, w.Code)
		}
	}
}
