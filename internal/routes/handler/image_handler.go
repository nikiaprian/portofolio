package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/media"
)

func parseResizeParams(c *gin.Context) (maxW, maxH, qual int) {
	if w := strings.TrimSpace(c.Query("w")); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			maxW = n
		}
	}
	if h := strings.TrimSpace(c.Query("h")); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 {
			maxH = n
		}
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			qual = n
		}
	}
	return maxW, maxH, qual
}

// ServeStaticOrResized serves files under /static/ and applies resize when ?w= / ?h= / ?q= are present.
func ServeStaticOrResized(c *gin.Context) {
	fp := strings.TrimSpace(c.Param("filepath"))
	if fp == "" {
		c.Status(http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(fp, "/") {
		fp = "/" + fp
	}
	staticPath := "/static" + fp
	diskPath := "./public/static" + fp

	maxW, maxH, qual := parseResizeParams(c)
	if maxW == 0 && maxH == 0 && qual == 0 {
		c.File(diskPath)
		return
	}
	if media.IsResizableAsset(staticPath) {
		serveResizedImageSrc(c, staticPath)
		return
	}
	c.File(diskPath)
}

// ServeResizedImage handles GET /img/uploads/.../file.webp?w=400&q=90 and legacy GET /img?src=...
func ServeResizedImage(c *gin.Context) {
	src := ""
	if path := strings.TrimPrefix(c.Param("filepath"), "/"); path != "" {
		raw := media.ResizeProxyPath + "/" + path
		if q := c.Request.URL.RawQuery; q != "" {
			raw += "?" + q
		}
		if p, ok := media.ParseResizeProxyURL(raw); ok {
			src = p
		}
	}
	if src == "" {
		src = strings.TrimSpace(c.Query("src"))
	}
	if src == "" {
		c.Status(http.StatusBadRequest)
		return
	}
	serveResizedImageSrc(c, src)
}

func serveResizedImageSrc(c *gin.Context, src string) {
	if media.IsExternalImageURL(src) {
		serveResizedRemoteImage(c, src)
		return
	}
	abs, rel, err := media.ResolvePublicFile(src)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid image source")
		return
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	if fi.IsDir() {
		c.Status(http.StatusBadRequest)
		return
	}

	maxW, maxH, qual := parseResizeParams(c)

	modTime := fi.ModTime().Unix()
	modSize := fi.Size()
	cacheKey := media.ResizeCacheKey(rel, modTime, modSize, maxW, maxH, qual)
	etag := media.ETag(cacheKey)

	if inm := strings.TrimSpace(c.GetHeader("If-None-Match")); inm != "" && inm == etag {
		c.Header("Cache-Control", resizeCacheControl(maxW, maxH, qual))
		c.Header("ETag", etag)
		c.Header("X-Image-Cache", "HIT")
		c.Status(http.StatusNotModified)
		return
	}

	cache := media.GlobalResizeCache()
	if data, ct, ok := cache.Get(cacheKey); ok {
		writeImageResponse(c, data, ct, etag, maxW, maxH, qual, "HIT")
		return
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	if maxW == 0 && maxH == 0 && qual == 0 {
		ct := contentTypeByExt(abs)
		_ = cache.Set(cacheKey, raw, ct)
		writeImageResponse(c, raw, ct, etag, maxW, maxH, qual, "MISS")
		return
	}

	mime := contentTypeByExt(abs)
	out, ct, err := media.DeliverResize(raw, mime, maxW, maxH, qual)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	_ = cache.Set(cacheKey, out, ct)
	writeImageResponse(c, out, ct, etag, maxW, maxH, qual, "MISS")
}

func serveResizedRemoteImage(c *gin.Context, imageURL string) {
	maxW, maxH, qual := parseResizeParams(c)
	cacheKey := media.RemoteResizeCacheKey(imageURL, maxW, maxH, qual)
	etag := media.ETag(cacheKey)

	if inm := strings.TrimSpace(c.GetHeader("If-None-Match")); inm != "" && inm == etag {
		c.Header("Cache-Control", resizeCacheControl(maxW, maxH, qual))
		c.Header("ETag", etag)
		c.Header("X-Image-Cache", "HIT")
		c.Status(http.StatusNotModified)
		return
	}

	cache := media.GlobalResizeCache()
	if data, ct, ok := cache.Get(cacheKey); ok {
		writeImageResponse(c, data, ct, etag, maxW, maxH, qual, "HIT")
		return
	}

	raw, mime, err := media.FetchRemoteImageBytes(c.Request.Context(), imageURL)
	if err != nil {
		c.String(http.StatusBadGateway, "failed to fetch remote image")
		return
	}

	if maxW == 0 && maxH == 0 && qual == 0 {
		_ = cache.Set(cacheKey, raw, mime)
		writeImageResponse(c, raw, mime, etag, maxW, maxH, qual, "MISS")
		return
	}

	out, ct, err := media.DeliverResize(raw, mime, maxW, maxH, qual)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	_ = cache.Set(cacheKey, out, ct)
	writeImageResponse(c, out, ct, etag, maxW, maxH, qual, "MISS")
}

func writeImageResponse(c *gin.Context, data []byte, contentType, etag string, maxW, maxH, qual int, cacheStatus string) {
	c.Header("Cache-Control", resizeCacheControl(maxW, maxH, qual))
	c.Header("ETag", etag)
	c.Header("Vary", "Accept")
	c.Header("X-Image-Cache", cacheStatus)
	c.Data(http.StatusOK, contentType, data)
}

func resizeCacheControl(maxW, maxH, qual int) string {
	if maxW == 0 && maxH == 0 && qual == 0 {
		return "public, max-age=86400"
	}
	return "public, max-age=31536000, immutable"
}

func contentTypeByExt(path string) string {
	low := strings.ToLower(path)
	switch {
	case strings.HasSuffix(low, ".webp"):
		return "image/webp"
	case strings.HasSuffix(low, ".png"):
		return "image/png"
	case strings.HasSuffix(low, ".gif"):
		return "image/gif"
	case strings.HasSuffix(low, ".jpg"), strings.HasSuffix(low, ".jpeg"):
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}
