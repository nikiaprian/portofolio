package storage

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

var postImgSrcRE = regexp.MustCompile(`(?i)<img[^>]+src\s*=\s*["']([^"']+)["']`)

// CollectPostImageURLs returns unique image URLs from cover and inline <img src> in HTML content.
func CollectPostImageURLs(p *entity.Post) []string {
	if p == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	if p.Image != nil {
		add(*p.Image)
	}
	for _, m := range postImgSrcRE.FindAllStringSubmatch(p.Content, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	return out
}

// DeletePostObjectURLs deletes stored objects for the given post (best-effort; logs failures).
func DeletePostObjectURLs(ctx context.Context, store ObjectStore, p *entity.Post) {
	if store == nil || p == nil {
		return
	}
	for _, u := range CollectPostImageURLs(p) {
		if err := store.DeleteURL(ctx, u); err != nil {
			log.Printf("storage: delete post image %s: %v", u, err)
		}
	}
}
