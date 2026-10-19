package portfolio

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var emailPattern = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)

func NormalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func ValidateEmail(raw string) string {
	email := NormalizeEmail(raw)
	if len(email) < 6 || len(email) > 150 || !emailPattern.MatchString(email) {
		return "Email tidak valid."
	}
	return ""
}

func NormalizeWhatsApp(raw string) string {
	raw = strings.TrimSpace(raw)
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if strings.HasPrefix(digits, "0") {
		digits = "62" + digits[1:]
	}
	if digits == "" {
		return ""
	}
	return "+" + digits
}

func ValidateWhatsApp(raw string) string {
	normalized := NormalizeWhatsApp(raw)
	digits := strings.TrimPrefix(normalized, "+")
	if len(digits) < 8 || len(digits) > 15 {
		return "Nomor WhatsApp tidak valid."
	}
	return ""
}

func ValidatePersonName(raw string) string {
	name := strings.TrimSpace(raw)
	if len([]rune(name)) < 2 || len([]rune(name)) > 100 {
		return "Nama harus 2 sampai 100 karakter."
	}
	return ""
}

// SafeHTTPURL returns raw when it is an http(s) URL.
func SafeHTTPURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}

func WhatsAppLink(raw string) string {
	normalized := NormalizeWhatsApp(raw)
	digits := strings.TrimPrefix(normalized, "+")
	if len(digits) < 8 {
		return ""
	}
	return "https://wa.me/" + digits
}

func Initials(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		r := []rune(parts[0])
		if len(r) > 2 {
			r = r[:2]
		}
		return strings.ToUpper(string(r))
	}
	a := []rune(parts[0])
	b := []rune(parts[len(parts)-1])
	return strings.ToUpper(string([]rune{a[0], b[0]}))
}

func FormatIDR(amount int64) string {
	if amount < 0 {
		amount = 0
	}
	s := itoa(amount)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	if s != "" {
		parts = append([]string{s}, parts...)
	}
	return "Rp" + strings.Join(parts, ".")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
