package scraper

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
)

var markdownImageRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)`)

// BulkCoverImageFromHTML extracts the first usable image URL from an HTML fragment and stores it
// (resized/converted when possible) as a post image. Returns nil when no image is found or download fails.
//
// This is designed as a fallback for bulk-generated posts where the AI output already contains <img> tags.
// db may be nil; stored_image_* caps are applied when db is non-nil.
func BulkCoverImageFromHTML(ctx context.Context, db *gorm.DB, store storage.ObjectStore, htmlFrag, postSlug string) *string {
	postSlug = strings.TrimSpace(postSlug)
	if postSlug == "" || strings.TrimSpace(htmlFrag) == "" {
		return nil
	}

	candidates := firstImageCandidatesFromHTML(htmlFrag)
	if len(candidates) == 0 {
		return nil
	}

	client := &http.Client{Timeout: 22 * time.Second}
	storeMaxEdge, storeWebPQ := repositories.PostImageStoreLimitsFromDB(db)
	var firstValidRemote string
	for _, raw := range candidates {
		abs := normalizeCandidateImageURL(raw)
		if abs == "" {
			continue
		}
		u, err := url.Parse(abs)
		if err != nil || u == nil {
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		if firstValidRemote == "" {
			firstValidRemote = abs
		}
		if local, err := downloadRemotePostImage(ctx, client, store, abs, "", postSlug, storeMaxEdge, storeWebPQ); err == nil && local != nil && strings.TrimSpace(*local) != "" {
			return local
		}
	}
	if firstValidRemote != "" {
		// Last fallback: keep a usable remote URL even when local rehost fails
		// (e.g. anti-hotlink restrictions or transient download/storage issues).
		return &firstValidRemote
	}
	return nil
}

func firstImageCandidatesFromHTML(htmlFrag string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlFrag))
	if err != nil {
		return markdownImageCandidates(htmlFrag)
	}

	seen := make(map[string]struct{}, 8)
	out := make([]string, 0, 6)

	doc.Find("img, source").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		cands := []string{
			attrTrim(s, "src"),
			attrTrim(s, "data-src"),
			attrTrim(s, "data-original"),
			attrTrim(s, "data-lazy-src"),
		}
		if ss := attrTrim(s, "srcset"); ss != "" {
			cands = append(cands, firstURLFromSrcset(ss))
		}

		for _, c := range cands {
			c = normalizeCandidateImageURL(c)
			if c == "" {
				continue
			}
			key := strings.ToLower(c)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, c)
			if len(out) >= 6 {
				return false
			}
		}
		return true
	})

	for _, c := range markdownImageCandidates(htmlFrag) {
		c = normalizeCandidateImageURL(c)
		if c == "" {
			continue
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
		if len(out) >= 6 {
			break
		}
	}

	return out
}

func markdownImageCandidates(s string) []string {
	matches := markdownImageRe.FindAllStringSubmatch(s, 6)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

func attrTrim(s *goquery.Selection, name string) string {
	if s == nil {
		return ""
	}
	v, ok := s.Attr(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// firstURLFromSrcset returns the first URL candidate from a srcset attribute value.
// Example: "a.webp 1x, b.webp 2x" -> "a.webp"
func firstURLFromSrcset(srcset string) string {
	srcset = strings.TrimSpace(srcset)
	if srcset == "" {
		return ""
	}
	// Split by commas into candidates, then take the URL part of the first candidate.
	first := srcset
	if i := strings.Index(srcset, ","); i >= 0 {
		first = srcset[:i]
	}
	first = strings.TrimSpace(first)
	if first == "" {
		return ""
	}
	parts := strings.Fields(first)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func normalizeCandidateImageURL(raw string) string {
	u := strings.TrimSpace(raw)
	u = strings.Trim(u, `"'`)
	u = strings.TrimSpace(html.UnescapeString(u))
	if u == "" || strings.HasPrefix(strings.ToLower(u), "data:") {
		return ""
	}
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}
	return u
}

