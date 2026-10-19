package posthtml

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// EnsureExternalAnchorsNofollowBlank adds rel tokens (nofollow, noopener, noreferrer) and target=_blank
// to each <a href> in the fragment when the link is an absolute URL to a host other than siteBaseURL's host.
// Relative links, same-host links, and javascript:/mailto:/tel:/data: are left unchanged.
func EnsureExternalAnchorsNofollowBlank(fragmentHTML, siteBaseURL string) string {
	fh := fragmentHTML
	baseStr := strings.TrimSpace(siteBaseURL)
	if strings.TrimSpace(fh) == "" || baseStr == "" {
		return fh
	}
	baseU, err := url.Parse(baseStr)
	if err != nil || baseU.Host == "" {
		return fh
	}
	siteHost := normalizeHostKey(baseU.Host)

	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_bebii_post_root">` + fh + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return fh
	}
	doc.Find("#_bebii_post_root a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		href = strings.TrimSpace(href)
		if href == "" {
			return
		}
		low := strings.ToLower(href)
		if strings.HasPrefix(low, "javascript:") || strings.HasPrefix(low, "mailto:") ||
			strings.HasPrefix(low, "tel:") || strings.HasPrefix(low, "data:") {
			return
		}
		u, ok := parseAbsoluteLinkURL(href)
		if !ok {
			return
		}
		if normalizeHostKey(u.Host) == siteHost {
			return
		}
		rel, _ := s.Attr("rel")
		s.SetAttr("rel", mergeRelTokens(rel, "nofollow", "noopener", "noreferrer"))
		s.SetAttr("target", "_blank")
	})
	out, err := doc.Find("#_bebii_post_root").Html()
	if err != nil {
		return fh
	}
	return out
}

func parseAbsoluteLinkURL(href string) (*url.URL, bool) {
	u, err := url.Parse(href)
	if err != nil {
		return nil, false
	}
	if u.Host != "" {
		return u, true
	}
	if strings.HasPrefix(strings.TrimSpace(href), "//") {
		u2, err2 := url.Parse("https:" + strings.TrimSpace(href))
		if err2 != nil || u2.Host == "" {
			return nil, false
		}
		return u2, true
	}
	return nil, false
}

func normalizeHostKey(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}

func mergeRelTokens(existing string, add ...string) string {
	have := map[string]string{}
	var out []string
	for _, p := range strings.Fields(existing) {
		lp := strings.ToLower(strings.TrimSpace(p))
		if lp == "" {
			continue
		}
		if _, ok := have[lp]; !ok {
			have[lp] = p
			out = append(out, p)
		}
	}
	for _, a := range add {
		la := strings.ToLower(strings.TrimSpace(a))
		if la == "" {
			continue
		}
		if _, ok := have[la]; !ok {
			have[la] = a
			out = append(out, a)
		}
	}
	return strings.Join(out, " ")
}
