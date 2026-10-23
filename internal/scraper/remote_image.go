package scraper

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/media"
	"github.com/nikiaprian/tirtayasa/internal/storage"
)

const (
	scrapedImageMaxBytes = 5 * 1024 * 1024
	scrapedImageDir      = "public/static/uploads/posts"
	scrapedImageURLBase  = "/static/uploads/posts"
	scrapedSlugMaxLen    = 220
)

var scrapedImageSlugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func scrapedImageExtForMime(mimeType string) (string, bool) {
	switch mimeType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

func newScrapedImageFilename(ext string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]) + ext, nil
}

func filenameForScrapedPost(slug, ext string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || len(slug) > scrapedSlugMaxLen || !scrapedImageSlugRe.MatchString(slug) {
		return newScrapedImageFilename(ext)
	}
	return slug + ext, nil
}

// downloadRemotePostImage downloads and stores a remote post image (resized, WebP when possible).
// If store is configured, uploads to object storage under posts/{slug}{ext} and returns absolute public URL.
// Otherwise, saves under public/static/uploads/posts and returns /static/uploads/posts/... path.
// storeMaxLongestEdge and storeWebPQuality follow site_settings stored_image_* (same as AI pipeline).
// Raster images are stored as WebP when possible (lossy for JPEG, lossless for PNG/GIF/WebP),
// with fallback to the original family if WebP encoding fails.
func downloadRemotePostImage(ctx context.Context, client *http.Client, store storage.ObjectStore, imageURL, referer, postSlug string, storeMaxLongestEdge, storeWebPQuality int) (*string, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return nil, fmt.Errorf("empty image url")
	}
	low := strings.ToLower(imageURL)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return nil, fmt.Errorf("unsupported image scheme")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tirtayasa-news-scraper/1.2; +https://tirtayasa.id)")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	res, err := client.Do(req)
	if err != nil {
		ScrapeLogf("warn", "", imageURL, "[scraper] GET image failed url=%s err=%v", imageURL, err)
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		LogHTTPNotOK(imageURL, res.StatusCode)
		return nil, fmt.Errorf("image status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, scrapedImageMaxBytes))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty image body")
	}

	mimeType := http.DetectContentType(body)
	if _, ok := scrapedImageExtForMime(mimeType); !ok {
		return nil, fmt.Errorf("unsupported image type %s", mimeType)
	}

	enc, err := media.EncodePostUploadWithStoreCap(body, mimeType, storeMaxLongestEdge, storeWebPQuality)
	if err != nil {
		return nil, err
	}

	filename, err := filenameForScrapedPost(postSlug, enc.Ext)
	if err != nil {
		return nil, err
	}

	if store != nil {
		key := "posts/" + filename
		url, err := store.Put(ctx, key, bytes.NewReader(enc.Data), enc.ContentType)
		if err != nil {
			return nil, err
		}
		return &url, nil
	}

	if err := os.MkdirAll(scrapedImageDir, 0o755); err != nil {
		return nil, err
	}
	targetPath := filepath.Join(scrapedImageDir, filename)
	if err := os.WriteFile(targetPath, enc.Data, 0o644); err != nil {
		return nil, err
	}

	urlPath := scrapedImageURLBase + "/" + filename
	return &urlPath, nil
}
