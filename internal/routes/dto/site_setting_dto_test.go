package dto

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUpdateSiteSettingRequest_articleSlugModeCategoryNested(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body, err := json.Marshal(map[string]any{
		"site_name":         "Tirtayasa",
		"default_title":     "Home",
		"article_slug_mode": "category_nested",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(nil)
	c.Request, _ = http.NewRequest(http.MethodPut, "/admin/site-setting", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var req UpdateSiteSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if req.ArticleSlugMode != "category_nested" {
		t.Fatalf("got %q", req.ArticleSlugMode)
	}
}
