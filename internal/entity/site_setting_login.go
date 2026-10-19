package entity

import (
	"path"
	"strings"
)

// EffectiveLoginHref returns the URL browsers and redirects should use for signing in.
// Empty raw means "/login". Relative paths must start with "/"; leading slash form only.
// http(s) URLs are returned unchanged. Unsafe or invalid values fall back to "/login".
func EffectiveLoginHref(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "/login"
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return s
	}
	if !strings.HasPrefix(s, "/") || strings.ContainsAny(s, " \t\n\r") {
		return "/login"
	}
	clean := path.Clean(s)
	if clean == "." || clean == "" {
		return "/login"
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	pfx := strings.ToLower(clean)
	if strings.HasPrefix(pfx, "/admin") || strings.HasPrefix(pfx, "/api/") || strings.HasPrefix(pfx, "/static/") {
		return "/login"
	}
	return clean
}

// MultiSegmentLoginRoutePaths returns internal paths with 2+ URL segments (e.g. /akun/masuk) that must be
// registered explicitly on the router. Single-segment paths (e.g. /masuk) are handled in the /:slug handler.
func MultiSegmentLoginRoutePaths(raw string) []string {
	h := EffectiveLoginHref(raw)
	low := strings.ToLower(h)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return nil
	}
	if h == "/login" {
		return nil
	}
	inner := strings.Trim(h, "/")
	if inner == "" || !strings.Contains(inner, "/") {
		return nil
	}
	return []string{"/" + inner}
}

// IsAdminOrLoginFrontendPath reports whether path is the SSR login page or /admin panel.
func IsAdminOrLoginFrontendPath(path, loginURLRaw string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return false
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	low := strings.ToLower(p)
	if low == "/admin" || strings.HasPrefix(low, "/admin/") {
		return true
	}
	loginHref := EffectiveLoginHref(loginURLRaw)
	loginLow := strings.ToLower(loginHref)
	if strings.HasPrefix(loginLow, "http://") || strings.HasPrefix(loginLow, "https://") {
		return false
	}
	if low == loginLow || strings.HasPrefix(low, loginLow+"/") {
		return true
	}
	for _, extra := range MultiSegmentLoginRoutePaths(loginURLRaw) {
		extraLow := strings.ToLower(extra)
		if low == extraLow || strings.HasPrefix(low, extraLow+"/") {
			return true
		}
	}
	return false
}
