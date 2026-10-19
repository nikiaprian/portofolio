package posthtml

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// EnsureInternalAnchorsAbsolute prefixes site-relative href values (/) with siteBaseURL.
// Absolute http(s)://, //, #, mailto:, tel:, and javascript: links are left unchanged.
func EnsureInternalAnchorsAbsolute(fragmentHTML, siteBaseURL string) string {
	fh := strings.TrimSpace(fragmentHTML)
	baseStr := strings.TrimRight(strings.TrimSpace(siteBaseURL), "/")
	if fh == "" || baseStr == "" {
		return fragmentHTML
	}

	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_bebii_post_root">` + fh + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return fragmentHTML
	}

	doc.Find("#_bebii_post_root a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" {
			return
		}
		low := strings.ToLower(href)
		if strings.HasPrefix(href, "#") ||
			strings.HasPrefix(low, "javascript:") ||
			strings.HasPrefix(low, "mailto:") ||
			strings.HasPrefix(low, "tel:") ||
			strings.HasPrefix(low, "data:") {
			return
		}
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(href, "//") {
			return
		}
		if !strings.HasPrefix(href, "/") {
			return
		}
		s.SetAttr("href", baseStr+href)
	})

	out, err := doc.Find("#_bebii_post_root").Html()
	if err != nil {
		return fragmentHTML
	}
	return out
}
