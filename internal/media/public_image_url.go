package media

import "strings"

func normalizePublicImageSrc(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return s
	}
	if strings.HasPrefix(s, "/") {
		return s
	}
	return "/" + s
}

// PublicAssetAbsoluteURL makes a site-relative or protocol-relative asset URL absolute.
func PublicAssetAbsoluteURL(baseURL, raw string) string {
	raw = strings.TrimSpace(StripResizePrefixes(raw))
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasPrefix(raw, "//") {
		if strings.HasPrefix(strings.ToLower(base), "https://") {
			return "https:" + raw
		}
		return "http:" + raw
	}
	if base == "" {
		return raw
	}
	return base + "/" + strings.TrimLeft(raw, "/")
}

// absolutizePublicAPIAssetURL makes a path absolute without unwrapping /img/ resize proxies.
func absolutizePublicAPIAssetURL(mediaBaseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}
	base := strings.TrimRight(strings.TrimSpace(mediaBaseURL), "/")
	if strings.HasPrefix(raw, "//") {
		if strings.HasPrefix(strings.ToLower(base), "https://") {
			return "https:" + raw
		}
		return "http:" + raw
	}
	if base == "" {
		return raw
	}
	return base + "/" + strings.TrimLeft(raw, "/")
}

// PublicAPIImageURL returns an absolute image URL for public JSON APIs (posts, authors).
// When self-hosted resize is disabled, the original CDN/static URL is returned.
// When enabled, external images use /img/{host}/{path} and local uploads use /static/... (no ?w=&h=; client supplies size).
func PublicAPIImageURL(mediaBaseURL, raw string, resizeEnabled bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	src := normalizePublicImageSrc(StripResizePrefixes(raw))
	if src == "" {
		return ""
	}
	if !resizeEnabled {
		return absolutizePublicAPIAssetURL(mediaBaseURL, src)
	}
	proxy := BuildImageProxyPath(src)
	if strings.TrimSpace(proxy) == "" {
		return ""
	}
	return absolutizePublicAPIAssetURL(mediaBaseURL, proxy)
}

// PublicFeedImageURL returns an absolute RSS/API thumbnail URL with optional self-hosted resize.
func PublicFeedImageURL(mediaBaseURL, raw string, resizeEnabled bool, width, quality string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	src := normalizePublicImageSrc(StripResizePrefixes(raw))
	if src == "" {
		return ""
	}
	if !resizeEnabled {
		return PublicAssetAbsoluteURL(mediaBaseURL, src)
	}
	resized := BuildResizeURL(src, strings.TrimSpace(width), "", strings.TrimSpace(quality))
	if strings.TrimSpace(resized) == "" {
		return ""
	}
	return PublicAssetAbsoluteURL(mediaBaseURL, resized)
}
