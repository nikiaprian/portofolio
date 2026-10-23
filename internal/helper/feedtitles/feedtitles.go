// Package feedtitles extracts article titles from common feed / export XML (RSS 2.0, Atom, WordPress WXR),
// WordPress/SEO post sitemaps (urlset + loc), and sitemap indexes (follows a few child sitemaps).
package feedtitles

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nikiaprian/tirtayasa/internal/helper/scraperguard"
)

const (
	maxXMLBytes  = 4 << 20 // 4 MiB
	maxTitles    = 1000
	fetchTimeout = 25 * time.Second
)

var (
	reRSSItem    = regexp.MustCompile(`(?is)<item\b[^>]*>([\s\S]*?)</item>`)
	reAtomEntry  = regexp.MustCompile(`(?is)<entry\b[^>]*>([\s\S]*?)</entry>`)
	reBlockTitle = regexp.MustCompile(`(?is)<title[^>]*>(?:\s*<!\[CDATA\[([\s\S]*?)\]\]>|([^<]*))</title>`)
	// WordPress / Yoast post sitemap: <url><loc>https://host/…/slug/</loc>…</url> (first <loc> per block is the page)
	reSitemapURL = regexp.MustCompile(`(?is)<url\b[^>]*>([\s\S]*?)</url>`)
	reLocTag     = regexp.MustCompile(`(?is)<loc>\s*([^<\s]+)\s*</loc>`)
	// Sitemap index: <sitemap><loc>https://host/post-sitemap-0.xml</loc></sitemap>
	reSitemapIndex = regexp.MustCompile(`(?is)<sitemap\b[^>]*>([\s\S]*?)</sitemap>`)
)

type rssRoot struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title string `xml:"title"`
}

type atomRoot struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title string `xml:"title"`
}

// ExtractFromBytes returns distinct titles in document order (max maxTitles).
func ExtractFromBytes(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	if len(raw) > maxXMLBytes {
		return nil, fmt.Errorf("XML is too large (max %d bytes)", maxXMLBytes)
	}
	if scraperguard.ContainsMarkerBytes(raw) {
		return nil, fmt.Errorf("feed is from the same Tirtayasa system; skipped")
	}
	safe := sanitizeAmpersandsForXML(string(raw))
	b := []byte(safe)

	var rss rssRoot
	if err := xml.Unmarshal(b, &rss); err == nil && len(rss.Channel.Items) > 0 {
		out := collectFromRSSItems(rss.Channel.Items)
		if len(out) > 0 {
			return dedupeLimit(out), nil
		}
	}

	var atom atomRoot
	if err := xml.Unmarshal(b, &atom); err == nil && len(atom.Entries) > 0 {
		out := make([]string, 0, len(atom.Entries))
		for _, e := range atom.Entries {
			if t := normalizeTitle(e.Title); t != "" {
				out = append(out, t)
			}
		}
		if len(out) > 0 {
			return dedupeLimit(out), nil
		}
	}

	if out := extractSitemapURLSetTitles(b); len(out) > 0 {
		return dedupeLimit(out), nil
	}

	out := extractViaItemRegex(b)
	if len(out) == 0 {
		out = extractViaEntryRegex(b)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no titles found: use RSS/Atom, WordPress WXR, or a post sitemap URL (urlset with <loc> page URLs)")
	}
	return dedupeLimit(out), nil
}

func extractSitemapURLSetTitles(b []byte) []string {
	var out []string
	for _, m := range reSitemapURL.FindAllSubmatch(b, -1) {
		if len(m) < 2 {
			continue
		}
		sm := reLocTag.FindSubmatch(m[1])
		if len(sm) < 2 {
			continue
		}
		loc := strings.TrimSpace(string(sm[1]))
		if t := titleFromSitemapPageURL(loc); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// extractSitemapIndexChildURLs returns <loc> targets inside <sitemap>…</sitemap> blocks (sitemap index files).
func extractSitemapIndexChildURLs(b []byte) []string {
	var out []string
	for _, m := range reSitemapIndex.FindAllSubmatch(b, -1) {
		if len(m) < 2 {
			continue
		}
		sm := reLocTag.FindSubmatch(m[1])
		if len(sm) < 2 {
			continue
		}
		loc := strings.TrimSpace(string(sm[1]))
		if loc != "" {
			out = append(out, loc)
		}
	}
	return out
}

func titleFromSitemapPageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return ""
	}
	segs := strings.Split(path, "/")
	last := segs[len(segs)-1]
	if last == "" && len(segs) >= 2 {
		last = segs[len(segs)-2]
	}
	last, err = url.PathUnescape(last)
	if err != nil {
		last = segs[len(segs)-1]
	}
	last = strings.TrimSuffix(last, ".html")
	last = strings.TrimSuffix(last, ".htm")
	if last == "" || strings.EqualFold(last, "feed") || strings.EqualFold(last, "page") {
		return ""
	}
	// Skip obvious non-post segments (year-only paths already handled by using last segment)
	t := strings.ReplaceAll(last, "-", " ")
	t = strings.ReplaceAll(t, "_", " ")
	return normalizeTitle(t)
}

func collectFromRSSItems(items []rssItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if t := normalizeTitle(it.Title); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func extractViaItemRegex(b []byte) []string {
	var out []string
	for _, m := range reRSSItem.FindAllSubmatch(b, -1) {
		if len(m) < 2 {
			continue
		}
		if t := firstTitleInBlock(m[1]); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func extractViaEntryRegex(b []byte) []string {
	var out []string
	for _, m := range reAtomEntry.FindAllSubmatch(b, -1) {
		if len(m) < 2 {
			continue
		}
		if t := firstTitleInBlock(m[1]); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func firstTitleInBlock(block []byte) string {
	sm := reBlockTitle.FindSubmatch(block)
	if len(sm) < 2 {
		return ""
	}
	if len(sm) >= 2 && len(sm[1]) > 0 {
		return normalizeTitle(string(sm[1]))
	}
	if len(sm) >= 3 && len(sm[2]) > 0 {
		return normalizeTitle(string(sm[2]))
	}
	return ""
}

func normalizeTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = html.UnescapeString(s)
	if utf8.RuneCountInString(s) > 500 {
		s = string([]rune(s)[:500])
	}
	return s
}

func dedupeLimit(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, t := range in {
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
		if len(out) >= maxTitles {
			break
		}
	}
	return out
}

// FetchAndExtract performs GET on feedURL (with SSRF checks), reads a limited body, and extracts titles.
func FetchAndExtract(client *http.Client, feedURL string) ([]string, error) {
	u, err := ValidateFeedURL(feedURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TirtayasaBlogBulkFeedTitles/1.0")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected HTTP status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxXMLBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxXMLBytes {
		return nil, fmt.Errorf("downloaded file is too large (max %d bytes)", maxXMLBytes)
	}
	if scraperguard.ContainsMarkerBytes(body) {
		return nil, fmt.Errorf("feed is from the same Tirtayasa system; skipped")
	}
	titles, err := ExtractFromBytes(body)
	if err == nil && len(titles) > 0 {
		return titles, nil
	}
	// Yoast/WP often serves post-sitemap.xml as a sitemap index pointing at post-sitemap-0.xml, etc.
	childURLs := extractSitemapIndexChildURLs(body)
	if len(childURLs) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("no titles found: use RSS/Atom, WXR, or a post sitemap with <url><loc>…</loc>")
	}
	const maxChild = 8
	var merged []string
	for i, child := range childURLs {
		if i >= maxChild {
			break
		}
		if _, ve := ValidateFeedURL(child); ve != nil {
			continue
		}
		sub, e := fetchURLBytes(client, child)
		if e != nil || len(sub) == 0 {
			continue
		}
		if scraperguard.ContainsMarkerBytes(sub) {
			continue
		}
		t2, e2 := ExtractFromBytes(sub)
		if e2 != nil || len(t2) == 0 {
			continue
		}
		merged = append(merged, t2...)
	}
	if len(merged) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("no titles found in sitemap index children (try a direct post-sitemap-0.xml URL)")
	}
	return dedupeLimit(merged), nil
}

func fetchURLBytes(client *http.Client, rawURL string) ([]byte, error) {
	u, err := ValidateFeedURL(rawURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TirtayasaBlogBulkFeedTitles/1.0")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected HTTP status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxXMLBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxXMLBytes {
		return nil, fmt.Errorf("downloaded file is too large (max %d bytes)", maxXMLBytes)
	}
	return body, nil
}

// ValidateFeedURL returns a normalized URL or an error if the target is not allowed.
func ValidateFeedURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are allowed")
	}
	host := strings.TrimSpace(strings.ToLower(u.Hostname()))
	if host == "" {
		return nil, fmt.Errorf("invalid URL")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("this host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("this address is not allowed")
		}
		return u, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("could not resolve host: %w", err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("host resolved to no addresses")
	}
	for _, a := range addrs {
		if !isPublicIP(a.IP) {
			return nil, fmt.Errorf("this host resolves to a non-public address")
		}
	}
	return u, nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return true
}

// sanitizeAmpersandsForXML fixes common invalid bare "&" outside CDATA so encoding/xml can parse feeds.
func sanitizeAmpersandsForXML(s string) string {
	var b strings.Builder
	b.Grow(len(s) + len(s)/64)
	for {
		idx := strings.Index(s, "<![CDATA[")
		if idx == -1 {
			b.WriteString(sanitizeAmpSegment(s))
			break
		}
		b.WriteString(sanitizeAmpSegment(s[:idx]))
		rest := s[idx:]
		endIdx := strings.Index(rest, "]]>")
		if endIdx < 0 {
			b.WriteString(rest)
			break
		}
		endIdx += len("]]>")
		b.WriteString(rest[:endIdx])
		s = rest[endIdx:]
	}
	return b.String()
}

func sanitizeAmpSegment(s string) string {
	var out strings.Builder
	out.Grow(len(s) + 8)
	i := 0
	for i < len(s) {
		j := strings.IndexByte(s[i:], '&')
		if j < 0 {
			out.WriteString(s[i:])
			break
		}
		j += i
		out.WriteString(s[i:j])
		if okEntityEnd(s, j) {
			end := entityEndAt(s, j)
			out.WriteString(s[j:end])
			i = end
			continue
		}
		out.WriteString("&amp;")
		i = j + 1
	}
	return out.String()
}

func okEntityEnd(s string, amp int) bool {
	_, ok := entityEndAtOK(s, amp)
	return ok
}

func entityEndAt(s string, amp int) int {
	end, _ := entityEndAtOK(s, amp)
	return end
}

func entityEndAtOK(s string, amp int) (end int, ok bool) {
	if amp >= len(s) || s[amp] != '&' {
		return 0, false
	}
	if amp+1 >= len(s) {
		return 0, false
	}
	if s[amp+1] == '#' {
		i := amp + 2
		if i >= len(s) {
			return 0, false
		}
		if s[i] == 'x' || s[i] == 'X' {
			i++
			start := i
			for i < len(s) && isHexByte(s[i]) {
				i++
			}
			if i == start || i >= len(s) || s[i] != ';' {
				return 0, false
			}
			return i + 1, true
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start || i >= len(s) || s[i] != ';' {
			return 0, false
		}
		return i + 1, true
	}
	rel := strings.IndexByte(s[amp+1:], ';')
	if rel < 0 {
		return 0, false
	}
	semi := amp + 1 + rel
	name := s[amp+1 : semi]
	if len(name) == 0 || len(name) > 32 {
		return 0, false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return 0, false
		}
	}
	return semi + 1, true
}

func isHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
