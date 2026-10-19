package portfolio

import (
	"regexp"
	"strings"
	"time"
)

const (
	PendingHold  = 24 * time.Hour
	subdomainMin = 3
	subdomainMax = 30
)

var subdomainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,28}[a-z0-9])$`)

// NormalizeSubdomain lowercases a requested label.
func NormalizeSubdomain(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// ValidateSubdomain reports why a label cannot be registered. Empty means the shape is valid.
func ValidateSubdomain(raw string) string {
	sub := NormalizeSubdomain(raw)
	if len(sub) < subdomainMin || len(sub) > subdomainMax {
		return "Subdomain harus 3 sampai 30 karakter."
	}
	if !subdomainPattern.MatchString(sub) {
		return "Subdomain hanya huruf kecil, angka, dan tanda hubung, dan tidak boleh diawali atau diakhiri hubung."
	}
	if IsReservedSubdomain(sub) {
		return "Subdomain ini tidak bisa dipakai."
	}
	return ""
}

func IsReservedSubdomain(sub string) bool {
	_, ok := reservedSubdomains[NormalizeSubdomain(sub)]
	return ok
}

// SubdomainHeld reports whether a portfolio row still blocks a new purchase.
func SubdomainHeld(status string, created, now time.Time) bool {
	switch status {
	case "active", "suspended":
		return true
	case "pending":
		return now.Sub(created) < PendingHold
	default:
		return true
	}
}
