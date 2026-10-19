package handler

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/Bebii-Digital/bebii-blog/internal/media"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
)

const wpImageMaxBytes = 10 << 20 // 10 MB

var wpImgTagRe = regexp.MustCompile(`(?i)<img\s[^>]*?src\s*=\s*["']([^"']+)["'][^>]*?>`)
var wpSrcSetRe = regexp.MustCompile(`(?i)\bsrcset\s*=\s*["']([^"']+)["']`)

var wpImageHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// buildAttachmentMap builds a map of wp_posts attachment ID -> GUID URL.
func buildAttachmentMap(posts []WpPost) map[int]string {
	m := make(map[int]string)
	for _, p := range posts {
		if p.PostType == "attachment" && strings.TrimSpace(p.Guid) != "" {
			m[p.ID] = strings.TrimSpace(p.Guid)
		}
	}
	return m
}

// resolveAttachmentURL resolves a WordPress attachment ID to its GUID URL.
func resolveAttachmentURL(thumbnailID string, attachmentMap map[int]string) string {
	thumbnailID = strings.TrimSpace(thumbnailID)
	if thumbnailID == "" {
		return ""
	}
	id := atoi(thumbnailID)
	if id == 0 {
		return ""
	}
	return attachmentMap[id]
}

// downloadWPImage downloads an image from rawURL, encodes to WebP, and saves to the ObjectStore.
// Returns the public URL of the saved image, or empty string on failure.
func downloadWPImage(ctx context.Context, rawURL string, store storage.ObjectStore, slug string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	body, contentType, err := fetchWPImage(ctx, rawURL)
	if err != nil {
		log.Printf("[wp-import] download image failed %s: %v", rawURL, err)
		return ""
	}

	// Detect MIME from body if header was empty
	if contentType == "" || !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		contentType = http.DetectContentType(body)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		log.Printf("[wp-import] not an image: %s (content-type: %s)", rawURL, contentType)
		return ""
	}

	// Normalize MIME for encoder
	inputMIME := normalizeWPMIME(contentType)

	// Encode to WebP
	encoded, err := media.EncodePostUpload(body, inputMIME)
	if err != nil {
		log.Printf("[wp-import] encode failed %s: %v", rawURL, err)
		// Fallback: save original format
		ext := extForMIME(inputMIME)
		filename := wpImageFilename(slug, ext)
		url, err := store.Put(ctx, "posts/"+filename, bytes.NewReader(body), contentType)
		if err != nil {
			log.Printf("[wp-import] store original failed %s: %v", rawURL, err)
			return ""
		}
		return url
	}

	filename := wpImageFilename(slug, encoded.Ext)
	url, err := store.Put(ctx, "posts/"+filename, bytes.NewReader(encoded.Data), encoded.ContentType)
	if err != nil {
		log.Printf("[wp-import] store failed %s: %v", rawURL, err)
		return ""
	}
	return url
}

func fetchWPImage(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; bebii-blog-wp-import/1.0)")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

	res, err := wpImageHTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, "", fmt.Errorf("HTTP %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, wpImageMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("empty body")
	}
	if len(body) > wpImageMaxBytes {
		return nil, "", fmt.Errorf("image too large (>10MB)")
	}

	ct := strings.TrimSpace(res.Header.Get("Content-Type"))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return body, ct, nil
}

func normalizeWPMIME(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch {
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return "image/jpeg"
	case strings.Contains(ct, "png"):
		return "image/png"
	case strings.Contains(ct, "gif"):
		return "image/gif"
	case strings.Contains(ct, "webp"):
		return "image/webp"
	default:
		return ct
	}
}

func extForMIME(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func wpImageFilename(slug, ext string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		slug = fmt.Sprintf("wp-%d", time.Now().UnixNano()%1000000)
	}
	// Truncate slug
	if len(slug) > 180 {
		slug = slug[:180]
	}
	return fmt.Sprintf("%s-%d%s", slug, time.Now().UnixNano()%1000000, ext)
}

// rewriteContentImages downloads images found in HTML content and replaces their URLs with local copies.
// Returns the rewritten content and the number of images downloaded.
func rewriteContentImages(ctx context.Context, content string, store storage.ObjectStore, baseDomain string) (string, int) {
	if strings.TrimSpace(content) == "" {
		return content, 0
	}

	// Track downloaded images by original URL to avoid re-downloading
	downloaded := make(map[string]string) // original URL -> new URL
	count := 0

	// Process <img> tags
	content = wpImgTagRe.ReplaceAllStringFunc(content, func(match string) string {
		src := extractSrcFromImgTag(match)
		if src == "" {
			return match
		}

		resolved := resolveWPContentURL(src, baseDomain)

		// Check cache
		if newURL, ok := downloaded[resolved]; ok {
			return strings.Replace(match, src, newURL, 1)
		}

		// Download and save
		slug := slugFromURL(resolved)
		newURL := downloadWPImage(ctx, resolved, store, slug)
		if newURL == "" {
			return match // keep original on failure
		}

		downloaded[resolved] = newURL
		count++
		return strings.Replace(match, src, newURL, 1)
	})

	return content, count
}

func extractSrcFromImgTag(tag string) string {
	// Try src="..." or src='...'
	lower := strings.ToLower(tag)
	idx := strings.Index(lower, "src=")
	if idx < 0 {
		return ""
	}
	rest := tag[idx+4:]
	if len(rest) == 0 {
		return ""
	}
	quote := rest[0]
	if quote != '"' && quote != '\'' {
		// No quote: take until space
		end := strings.IndexByte(rest, ' ')
		if end < 0 {
			return strings.TrimSpace(rest)
		}
		return rest[:end]
	}
	end := strings.IndexByte(rest[1:], quote)
	if end < 0 {
		return ""
	}
	return rest[1 : end+1]
}

func resolveWPContentURL(src, baseDomain string) string {
	src = strings.TrimSpace(html.UnescapeString(src))
	if src == "" {
		return ""
	}

	// Already absolute HTTP URL
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return src
	}

	// Protocol-relative
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}

	// Relative path - resolve against base domain
	if baseDomain != "" {
		base := baseDomain
		if !strings.HasPrefix(base, "http") {
			base = "https://" + base
		}
		u, err := url.Parse(base)
		if err == nil {
			ref, err := url.Parse(src)
			if err == nil {
				return u.ResolveReference(ref).String()
			}
		}
	}

	return src
}

func slugFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	if base == "." || base == "/" || base == "" {
		return ""
	}
	// Remove extension
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	return wpSlugify(base)
}
