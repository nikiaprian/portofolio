package media

import (
	"net/url"
	"strconv"
	"strings"
)

// ResizeProxyPath is the HTTP path prefix for on-the-fly resized images (self-hosted).
const ResizeProxyPath = "/img"

// remoteResizePrefix is the first path segment for proxied external image URLs.
const remoteResizePrefix = "remote"

// Resizable path prefixes under public/static (uploads and site images only).
var resizablePathPrefixes = []string{
	"/static/uploads/",
	"/static/images/",
}

// NormalizeAssetPath returns a site-relative path (/static/...) from src, or "" if unsupported.
func NormalizeAssetPath(src string) string {
	s := StripResizePrefixes(strings.TrimSpace(src))
	if s == "" {
		return ""
	}
	if IsExternalImageURL(s) {
		return s
	}
	if strings.HasPrefix(s, "/") {
		return s
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		u, err := url.Parse(s)
		if err != nil || u == nil {
			return ""
		}
		p := u.EscapedPath()
		if p == "" {
			return ""
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		return p
	}
	return "/" + s
}

// IsResizableAsset reports whether src may be resized (local static or external http(s) image).
func IsResizableAsset(src string) bool {
	if IsExternalImageURL(src) {
		return true
	}
	p := NormalizeAssetPath(src)
	if p == "" {
		return false
	}
	for _, pref := range resizablePathPrefixes {
		if strings.HasPrefix(p, pref) {
			return true
		}
	}
	return false
}

// BuildResizeURL returns /img/...?w=&h=&q= for local assets or /img/{host}/{path}?w=&h=&q= for external URLs.
func BuildResizeURL(src, width, height, quality string) string {
	if strings.TrimSpace(width) == "" && strings.TrimSpace(height) == "" && strings.TrimSpace(quality) == "" {
		return src
	}
	s := StripResizePrefixes(strings.TrimSpace(src))
	if s == "" {
		return src
	}
	if IsExternalImageURL(s) {
		return buildRemoteResizeURL(s, width, height, quality)
	}
	base := NormalizeAssetPath(s)
	if base == "" || !strings.HasPrefix(base, "/static/") {
		return src
	}
	rel := strings.TrimPrefix(base, "/static/")
	if rel == "" {
		return src
	}
	qs := resizeQueryParams(width, height, quality)
	if len(qs) == 0 {
		return src
	}
	return ResizeProxyPath + "/" + escapePathSegments(rel) + "?" + strings.Join(qs, "&")
}

func buildRemoteResizeURL(rawURL, width, height, quality string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil || u.Host == "" {
		return rawURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return rawURL
	}
	qs := resizeQueryParams(width, height, quality)
	if len(qs) == 0 {
		return rawURL
	}
	rel := u.Host
	if p := strings.TrimPrefix(u.Path, "/"); p != "" {
		rel += "/" + p
	}
	return ResizeProxyPath + "/" + escapePathSegments(rel) + "?" + strings.Join(qs, "&")
}

func isLocalStaticResizeRel(rel string) bool {
	return strings.HasPrefix(rel, "uploads/") || strings.HasPrefix(rel, "images/")
}

func looksLikeRemoteHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" || host == "uploads" || host == "images" {
		return false
	}
	if strings.Contains(host, ".") || strings.Contains(host, ":") {
		return true
	}
	return host == "localhost"
}

func resizeQueryParams(width, height, quality string) []string {
	var qs []string
	if w := strings.TrimSpace(width); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			qs = append(qs, "w="+strconv.Itoa(n))
		}
	}
	if h := strings.TrimSpace(height); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 {
			qs = append(qs, "h="+strconv.Itoa(n))
		}
	}
	if qual := strings.TrimSpace(quality); qual != "" {
		if n, err := strconv.Atoi(qual); err == nil && n > 0 && n <= 100 {
			qs = append(qs, "q="+strconv.Itoa(n))
		}
	}
	return qs
}

// ParseResizeProxyURL extracts the image source (local /static/... or full external URL).
func ParseResizeProxyURL(raw string) (source string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, ResizeProxyPath+"/") {
		u, err := url.Parse(s)
		if err != nil || u == nil {
			return "", false
		}
		rel := strings.TrimPrefix(u.Path, ResizeProxyPath+"/")
		if rel == "" {
			return "", false
		}
		rel, err = unescapePathSegments(rel)
		if err != nil || rel == "" {
			return "", false
		}
		if strings.HasPrefix(rel, remoteResizePrefix+"/") {
			return parseRemoteResizeRel(rel)
		}
		if isLocalStaticResizeRel(rel) {
			staticPath := "/static/" + rel
			if IsResizableAsset(staticPath) {
				return staticPath, true
			}
			return "", false
		}
		return parseHostPathRemoteRel(rel)
	}
	if strings.HasPrefix(low, ResizeProxyPath+"?") || strings.HasPrefix(low, ResizeProxyPath+"&") {
		u, err := url.Parse(s)
		if err != nil || u == nil {
			return "", false
		}
		inner := strings.TrimSpace(u.Query().Get("src"))
		if inner == "" {
			return "", false
		}
		if IsExternalImageURL(inner) {
			return inner, true
		}
		staticPath := NormalizeAssetPath(inner)
		if staticPath != "" && strings.HasPrefix(staticPath, "/static/") {
			return staticPath, true
		}
	}
	return "", false
}

func parseHostPathRemoteRel(rel string) (string, bool) {
	parts := strings.SplitN(rel, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	host := parts[0]
	if !looksLikeRemoteHost(host) {
		return "", false
	}
	path := "/" + strings.TrimPrefix(parts[1], "/")
	out := "https://" + host + path
	if !IsExternalImageURL(out) {
		return "", false
	}
	return out, true
}

func parseRemoteResizeRel(rel string) (string, bool) {
	rest := strings.TrimPrefix(rel, remoteResizePrefix+"/")
	if rest == "" || rest == rel {
		return "", false
	}
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return "", false
	}
	scheme := strings.ToLower(parts[0])
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	host := parts[1]
	if host == "" {
		return "", false
	}
	path := "/"
	if len(parts) > 2 && parts[2] != "" {
		path = "/" + parts[2]
	}
	out := scheme + "://" + host + path
	if !IsExternalImageURL(out) {
		return "", false
	}
	return out, true
}

// stripResizeFromAbsoluteURL unwraps http(s)://host/img/... back to /static/... or a remote target URL.
func stripResizeFromAbsoluteURL(s string) (string, bool) {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return "", false
	}
	u, err := url.Parse(s)
	if err != nil || u == nil {
		return "", false
	}
	path := u.Path
	if path == "" || !strings.HasPrefix(path, ResizeProxyPath+"/") {
		return "", false
	}
	proxyRaw := ResizeProxyPath + strings.TrimPrefix(path, ResizeProxyPath)
	if u.RawQuery != "" {
		proxyRaw += "?" + u.RawQuery
	}
	if inner, ok := ParseResizeProxyURL(proxyRaw); ok {
		return inner, true
	}
	return "", false
}

// StripResizePrefixes removes /img/... and legacy /img?src=... wrappers.
func StripResizePrefixes(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	if inner, ok := stripResizeFromAbsoluteURL(s); ok {
		return inner
	}
	if p, ok := ParseResizeProxyURL(s); ok {
		return p
	}
	low := strings.ToLower(s)
	const cf = "/cdn-cgi/image/"
	if idx := strings.Index(low, cf); idx >= 0 {
		rest := s[idx+len(cf):]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return s[:idx] + rest[slash:]
		}
	}
	return s
}

func escapePathSegments(rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func unescapePathSegments(rel string) (string, error) {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		dec, err := url.PathUnescape(p)
		if err != nil {
			return "", err
		}
		parts[i] = dec
	}
	return strings.Join(parts, "/"), nil
}
