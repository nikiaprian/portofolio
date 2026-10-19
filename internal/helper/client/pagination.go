package client

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// ListPageSize is the default page size for SSR category, /posts, and /author lists.
const ListPageSize = 12

// ParseListPage reads the ?page= query param for client SSR list pages.
func ParseListPage(c *gin.Context) int {
	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}
	return page
}

func canonicalListPath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return strings.TrimSuffix(basePath, "/")
}

func listPageURL(basePath string, page int) string {
	if page <= 1 {
		return canonicalListPath(basePath)
	}
	return canonicalListPath(basePath) + "?page=" + strconv.Itoa(page)
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// compactPaginationSegments returns page numbers to show; -1 means ellipsis.
func compactPaginationSegments(current, totalPages int) []int {
	if totalPages <= 5 {
		out := make([]int, totalPages)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}
	current = intMax(1, intMin(current, totalPages))
	var out []int
	if current <= 3 {
		out = append(out, 1, 2, 3, -1, totalPages)
		return out
	}
	if current >= totalPages-2 {
		out = append(out, 1, -1, totalPages-2, totalPages-1, totalPages)
		return out
	}
	out = append(out, 1, -1, current-1, current, current+1, -1, totalPages)
	return out
}

// BuildPaginationLinks builds rows for the client-pagination partial (prev, pages, next).
func BuildPaginationLinks(current, totalPages int, basePath string) []gin.H {
	if totalPages <= 1 {
		return nil
	}
	current = intMax(1, intMin(current, totalPages))
	basePath = canonicalListPath(basePath)
	segs := compactPaginationSegments(current, totalPages)

	var links []gin.H
	if current <= 1 {
		links = append(links, gin.H{"kind": "prev", "label": "←", "href": "", "disabled": true, "active": false})
	} else {
		links = append(links, gin.H{"kind": "prev", "label": "←", "href": listPageURL(basePath, current-1), "disabled": false, "active": false})
	}
	for _, seg := range segs {
		if seg < 0 {
			links = append(links, gin.H{"kind": "ellipsis", "label": "…", "href": "", "disabled": false, "active": false})
			continue
		}
		links = append(links, gin.H{
			"kind":     "page",
			"label":    strconv.Itoa(seg),
			"href":     listPageURL(basePath, seg),
			"disabled": false,
			"active":   seg == current,
		})
	}
	if current >= totalPages {
		links = append(links, gin.H{"kind": "next", "label": "→", "href": "", "disabled": true, "active": false})
	} else {
		links = append(links, gin.H{"kind": "next", "label": "→", "href": listPageURL(basePath, current+1), "disabled": false, "active": false})
	}
	return links
}

// CategoryPublicWebPath returns the canonical client URL path for a category (nested when subcategory).
func CategoryPublicWebPath(cat entity.Category) string {
	return entity.CategoryPublicWebPath(cat)
}

// AuthorPublicListPath returns the base list path for an author page (/author/:slug).
func AuthorPublicListPath(slug string) string {
	s := strings.TrimSpace(slug)
	if s == "" {
		return "/author"
	}
	return "/author/" + url.PathEscape(s)
}
