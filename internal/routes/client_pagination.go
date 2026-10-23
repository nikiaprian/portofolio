package routes

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// clientListPageSize is the default page size for SSR category, /posts, and /author lists.
const clientListPageSize = 12

func parseClientListPage(c *gin.Context) int {
	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}
	return page
}

func clientCanonicalListPath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return strings.TrimSuffix(basePath, "/")
}

func clientListPageURL(basePath string, page int) string {
	if page <= 1 {
		return clientCanonicalListPath(basePath)
	}
	return clientCanonicalListPath(basePath) + "?page=" + strconv.Itoa(page)
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
// Pola ringkas: mis. 1 2 3 … 50 atau 1 … 10 11 12 … 50 (maks. ~7 slot angka + ellipsis).
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

// buildClientPaginationLinks builds rows for the client-pagination partial (prev, pages, next).
func buildClientPaginationLinks(current, totalPages int, basePath string) []gin.H {
	if totalPages <= 1 {
		return nil
	}
	current = intMax(1, intMin(current, totalPages))
	basePath = clientCanonicalListPath(basePath)
	segs := compactPaginationSegments(current, totalPages)

	var links []gin.H
	if current <= 1 {
		links = append(links, gin.H{"kind": "prev", "label": "←", "href": "", "disabled": true, "active": false})
	} else {
		links = append(links, gin.H{"kind": "prev", "label": "←", "href": clientListPageURL(basePath, current-1), "disabled": false, "active": false})
	}
	for _, seg := range segs {
		if seg < 0 {
			links = append(links, gin.H{"kind": "ellipsis", "label": "…", "href": "", "disabled": false, "active": false})
			continue
		}
		links = append(links, gin.H{
			"kind":     "page",
			"label":    strconv.Itoa(seg),
			"href":     clientListPageURL(basePath, seg),
			"disabled": false,
			"active":   seg == current,
		})
	}
	if current >= totalPages {
		links = append(links, gin.H{"kind": "next", "label": "→", "href": "", "disabled": true, "active": false})
	} else {
		links = append(links, gin.H{"kind": "next", "label": "→", "href": clientListPageURL(basePath, current+1), "disabled": false, "active": false})
	}
	return links
}

// categoryPublicWebPath returns the canonical client URL path for a category (nested when subcategory).
func categoryPublicWebPath(cat entity.Category) string {
	return entity.CategoryPublicWebPath(cat)
}

// authorPublicListPath returns the base list path for an author page (/author/:slug).
func authorPublicListPath(slug string) string {
	s := strings.TrimSpace(slug)
	if s == "" {
		return "/author"
	}
	return "/author/" + url.PathEscape(s)
}
