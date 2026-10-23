package handler

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

func parsePagination(c *gin.Context) (page int, size int) {
	page = 1
	size = 10

	if raw := c.Query("page"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}

	if raw := c.Query("size"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			size = v
		}
	}

	// hard limits to prevent huge queries (admin CRUD page sizes)
	if size > 500 {
		size = 500
	}

	return page, size
}

func totalPages(total int64, size int) int {
	if size <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(size)))
}

// parsePublicPostsPagination parses page/size for the public post archive.
// Default page size is 12 when "size" is omitted.
func parsePublicPostsPagination(c *gin.Context) (page int, size int) {
	page = 1
	size = 12

	if raw := c.Query("page"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
		}
	}

	if raw := c.Query("size"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			size = v
		}
	}

	if size > 100 {
		size = 100
	}

	return page, size
}

// parsePublicSkip returns the optional skip query (rows to skip before pagination), default 0.
func parsePublicSkip(c *gin.Context) int {
	raw := c.Query("skip")
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// totalPagesWithSkip computes page count when the list starts after skipping `skip` rows of `total`.
func totalPagesWithSkip(total int64, skip int, size int) int {
	if size <= 0 {
		return 0
	}
	if skip < 0 {
		skip = 0
	}
	effective := total - int64(skip)
	if effective < 0 {
		effective = 0
	}
	return totalPages(effective, size)
}
