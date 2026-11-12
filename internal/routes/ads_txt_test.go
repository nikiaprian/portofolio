package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/nikiaprian/tirtayasa/config"
)

func chdirModuleRoot(t *testing.T) {
	t.Helper()
	start, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "public", "templates")); err == nil {
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(start) })
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("public/templates not found")
		}
		dir = parent
	}
}

func TestAdsTxtAndRobotsTxtDoNotRedirectHome(t *testing.T) {
	chdirModuleRoot(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterFrontendRoutes(r, nil, config.Config{})

	for _, path := range []string{"/ads.txt", "/robots.txt"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			req := httptest.NewRequest(method, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s %s status = %d, location = %q, body = %q", method, path, w.Code, w.Header().Get("Location"), w.Body.String())
			}
			if loc := w.Header().Get("Location"); loc != "" {
				t.Fatalf("%s %s redirected to %q", method, path, loc)
			}
			if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
				t.Fatalf("%s %s content-type = %q", method, path, ct)
			}
		}
	}
}
