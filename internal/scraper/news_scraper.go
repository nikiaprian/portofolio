package scraper

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	appconfig "github.com/nikiaprian/tirtayasa/config"
	"github.com/nikiaprian/tirtayasa/internal/ai"
	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/alsoread"
	"github.com/nikiaprian/tirtayasa/internal/helper/postmeta"
	"github.com/nikiaprian/tirtayasa/internal/helper/imagecaption"
	"github.com/nikiaprian/tirtayasa/internal/helper/scraperguard"
	"github.com/nikiaprian/tirtayasa/internal/helper/tagcontent"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/storage"
	"github.com/PuerkitoBio/goquery"
	htmlpkg "golang.org/x/net/html"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

// tagDisplayTitleCaser title-cases a single word for tag names (e.g. "dosen" → "Dosen").
var tagDisplayTitleCaser = cases.Title(language.Indonesian)

// scraperRunMu ensures only one scrape cycle runs at a time (background ticker + "run now"
// can otherwise overlap; dedupe is checked before AI but insert happens much later → duplicates).
var scraperRunMu sync.Mutex

func tagWordHasMixedCase(s string) bool {
	var hasUpper, hasLower bool
	for _, r := range s {
		if unicode.IsLetter(r) {
			if unicode.IsUpper(r) {
				hasUpper = true
			}
			if unicode.IsLower(r) {
				hasLower = true
			}
			if hasUpper && hasLower {
				return true
			}
		}
	}
	return false
}

// tagWordIsAllUpperLetters is true when every letter rune is uppercase (e.g. WFH, SD, NASA). Non-letters are ignored.
func tagWordIsAllUpperLetters(s string) bool {
	var hasLetter bool
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

// normalizeTagWord title-cases unless the token already looks intentional (iPhone, WFH, SD, 3D).
func normalizeTagWord(word string) string {
	word = strings.TrimSpace(word)
	if word == "" {
		return ""
	}
	if tagWordHasMixedCase(word) || tagWordIsAllUpperLetters(word) {
		return word
	}
	return strings.TrimSpace(tagDisplayTitleCaser.String(word))
}

const defaultScrapeInterval = 5 * time.Minute

// maxFeedBodyBytes limits response size when downloading feeds or HTML discovery pages.
const maxFeedBodyBytes = 8 << 20

// maxSitemapBytes allows larger WordPress sitemap payloads.
const maxSitemapBytes = 24 << 20

// RSS/Atom often only includes a short lead; fetch article HTML when plain text is shorter than this.
const minPlainCharsBeforeArticleFetch = 400

// minScrapedContentWords is the minimum word count (whitespace-separated tokens after stripping HTML)
// for scraped article text; shorter candidates are skipped and not persisted.
const minScrapedContentWords = 250

// maxArticleHTMLBytes caps HTML download size when extracting full article text.
const maxArticleHTMLBytes = 3 << 20

// maxArticlePageFetchesPerFeed avoids one slow run downloading hundreds of article pages.
const maxArticlePageFetchesPerFeed = 40

// maxTagsFromMetaKeywords limits how many keyword segments become post tags per scrape.
const maxTagsFromMetaKeywords = 3

// maxCreatedPostsPerDomainPerCategoryPerRun prevents the scraper from exploding
// output when multiple feeds/pages from the same domain contain lots of articles.
// It's expressed as "how many new posts are allowed per domain per category per run".
const maxCreatedPostsPerDomainPerCategoryPerRun = 1

const scraperHTTPMaxAttempts = 3

const scraperUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

var articleBodyClassHints = []string{
	"read__content", "detail_text", "detail__body", "article__content",
	"text_detail", "post-content", "entry-content", "article-content",
	"detail-body", "main-content",
	// WordPress / common themes
	"wp-block-post-content", "wp-block-group__inner-container", "the_content",
	"single-content", "single__content", "post__content", "post-body", "content-area", "site-content",
}

var (
	htmlTagRegex    = regexp.MustCompile("<[^>]*>")
	spaceSeqRegex   = regexp.MustCompile(`\s+`)
	nonSlugRegex    = regexp.MustCompile(`[^a-z0-9]+`)
	nonWordRegex    = regexp.MustCompile(`[^\p{L}\p{N}\s]+`)
	imgSrcRegex     = regexp.MustCompile(`(?is)<img[^>]+src\s*=\s*["']([^"']+)["']`)
	wpPostSitemapRE = regexp.MustCompile(`(?i)^(https?://.+?/post-sitemap)(\d+)(\.xml(?:\?.*)?)$`)
	// Match <link> tags that advertise RSS or Atom; attrs may appear in any order.
	rssAtomLinkRE = regexp.MustCompile(`(?is)<link\s+([^>]+)>`)
	hrefAttrRE    = regexp.MustCompile(`(?is)\bhref\s*=\s*["']([^"']+)["']`)
)

func doScraperGET(ctx context.Context, client *http.Client, targetURL string) (*http.Response, error) {
	u := strings.TrimSpace(targetURL)
	var lastErr error
	for attempt := 1; attempt <= scraperHTTPMaxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			ScrapeLogf("error", "", u, "[scraper] GET bad request url=%s err=%v", u, err)
			return nil, err
		}
		req.Header.Set("User-Agent", scraperUserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
		req.Header.Set("Cache-Control", "no-cache")

		res, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			if res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusForbidden || res.StatusCode >= http.StatusInternalServerError {
				lastErr = fmt.Errorf("transient status %d", res.StatusCode)
				res.Body.Close()
			} else {
				return res, nil
			}
		}
		if attempt == scraperHTTPMaxAttempts {
			break
		}

		backoff := time.Duration(attempt*attempt) * 400 * time.Millisecond
		jitter := time.Duration(rand.IntN(300)) * time.Millisecond
		wait := backoff + jitter
		select {
		case <-ctx.Done():
			ScrapeLogf("error", "", u, "[scraper] GET cancelled url=%s err=%v", u, ctx.Err())
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	if lastErr != nil {
		ScrapeLogf("error", "", u, "[scraper] GET failed url=%s err=%v", u, lastErr)
	}
	return nil, lastErr
}

func isProbablyHTML(body []byte, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml") {
		return true
	}
	s := strings.TrimLeft(string(bytes.TrimSpace(body)), "\ufeff")
	ls := strings.ToLower(s)
	return strings.HasPrefix(ls, "<!doctype html") ||
		strings.HasPrefix(ls, "<html") ||
		(strings.HasPrefix(ls, "<?xml") && strings.Contains(ls, "xmlns=") && strings.Contains(ls, "xhtml"))
}

// extractFeedURLFromHTML finds rel=alternate application/rss+xml or atom feed URL inside HTML.
func extractFeedURLFromHTML(pageURL string, body []byte) string {
	s := string(body)
	for _, m := range rssAtomLinkRE.FindAllStringSubmatch(s, -1) {
		if len(m) < 2 {
			continue
		}
		attrs := m[1]
		al := strings.ToLower(attrs)
		if !strings.Contains(al, `rel=`) || !strings.Contains(al, `alternate`) {
			continue
		}
		if !strings.Contains(al, "application/rss+xml") && !strings.Contains(al, "application/atom+xml") {
			continue
		}
		hm := hrefAttrRE.FindStringSubmatch(attrs)
		if len(hm) < 2 {
			continue
		}
		href := strings.TrimSpace(html.UnescapeString(hm[1]))
		if href == "" || href == "#" {
			continue
		}
		base, err := url.Parse(pageURL)
		if err != nil {
			return href
		}
		ref, err := url.Parse(href)
		if err != nil {
			return href
		}
		return base.ResolveReference(ref).String()
	}
	return ""
}

func fetchFeedBytes(ctx context.Context, client *http.Client, rawURL string, depth int) ([]byte, error) {
	if depth > 2 {
		return nil, fmt.Errorf("feed discovery depth exceeded")
	}
	feedURL := strings.TrimSpace(rawURL)
	if feedURL == "" {
		return nil, fmt.Errorf("empty url")
	}
	res, err := doScraperGET(ctx, client, feedURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		LogHTTPNotOK(feedURL, res.StatusCode)
		return nil, fmt.Errorf("unexpected status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxFeedBodyBytes))
	if err != nil {
		ScrapeLogf("warn", "", feedURL, "[scraper] GET feed read body failed url=%s err=%v", feedURL, err)
		return nil, err
	}
	ct := res.Header.Get("Content-Type")
	if isProbablyHTML(body, ct) {
		if next := extractFeedURLFromHTML(feedURL, body); next != "" && next != feedURL {
			return fetchFeedBytes(ctx, client, next, depth+1)
		}
		return nil, fmt.Errorf("url returned html, no rss/atom link found (use direct rss.xml /feed url)")
	}
	return body, nil
}

// sanitizeXMLAmpersands escapes bare "&" that are not valid XML character references / entities.
// Many news feeds ship HTML snippets with unescaped "&" which breaks encoding/xml.
func sanitizeXMLAmpersandsPreservingCDATA(s string) string {
	var out strings.Builder
	out.Grow(len(s) + len(s)/64)
	for {
		idx := strings.Index(s, "<![CDATA[")
		if idx == -1 {
			out.WriteString(sanitizeXMLAmpersands(s))
			break
		}
		out.WriteString(sanitizeXMLAmpersands(s[:idx]))
		rest := s[idx:]
		endIdx := strings.Index(rest, "]]>")
		if endIdx < 0 {
			out.WriteString(rest)
			break
		}
		endIdx += len("]]>")
		out.WriteString(rest[:endIdx])
		s = rest[endIdx:]
	}
	return out.String()
}

func sanitizeXMLAmpersands(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	i := 0
	for i < len(s) {
		j := strings.IndexByte(s[i:], '&')
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		j += i
		b.WriteString(s[i:j])
		end, ok := xmlEntityEnd(s, j)
		if ok {
			b.WriteString(s[j:end])
			i = end
			continue
		}
		b.WriteString("&amp;")
		i = j + 1
	}
	return b.String()
}

func xmlEntityEnd(s string, amp int) (end int, ok bool) {
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
			for i < len(s) && isXMLHexByte(s[i]) {
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
	i := amp + 1
	r, wid := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && wid == 1 {
		return 0, false
	}
	if !unicode.IsLetter(r) && r != '_' && r != ':' {
		return 0, false
	}
	i += wid
	for i < len(s) {
		r, w := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && w == 0 {
			break
		}
		if r == ';' {
			return i + w, true
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' || r == ':') {
			break
		}
		i += w
	}
	return 0, false
}

func isXMLHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

type rssDocument struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type rssItem struct {
	Title string `xml:"title"`
	// Link is often the canonical article URL; Guid may duplicate or hold permalink.
	Link           string `xml:"link"`
	GUID           string `xml:"guid"`
	Description    string `xml:"description"`
	ContentEncoded string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	PubDate        string `xml:"pubDate"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published"`
	Links     []atomLink `xml:"link"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type sitemapURLSet struct {
	URLs []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

type sitemapIndex struct {
	Sitemaps []sitemapRef `xml:"sitemap"`
}

type sitemapRef struct {
	Loc string `xml:"loc"`
}

func rssItemLink(it rssItem) string {
	if u := strings.TrimSpace(it.Link); strings.HasPrefix(u, "http") {
		return u
	}
	if u := strings.TrimSpace(it.GUID); strings.HasPrefix(u, "http") {
		return u
	}
	return strings.TrimSpace(it.Link)
}

func rssItemBodyHTML(ctx context.Context, client *http.Client, it rssItem, link string, fetchesLeft int) (raw string, hero string, metaKeywords string, fetchesLeftOut int) {
	raw = firstNonEmpty(it.ContentEncoded, it.Description)
	if scraperguard.ContainsMarkerString(raw) {
		return raw, "", "", fetchesLeft
	}
	hero = strings.TrimSpace(postmeta.FirstImageURL(raw))
	fetchesLeftOut = fetchesLeft

	// If the feed doesn't provide an image, try to fetch meta (og/twitter) without rewriting content.
	// This helps feeds that include full text but omit images in the payload.
	if hero == "" && fetchesLeftOut > 0 && strings.HasPrefix(link, "http") {
		fetchesLeftOut--
		if og, kw, err := fetchArticleMeta(ctx, client, link); err == nil {
			if hero == "" {
				hero = strings.TrimSpace(og)
			}
			if metaKeywords == "" {
				metaKeywords = strings.TrimSpace(kw)
			}
		}
	}

	if fetchesLeftOut > 0 && plainTextRuneLen(raw) < minPlainCharsBeforeArticleFetch && strings.HasPrefix(link, "http") {
		fetchesLeftOut--
		if plain, og, kw, err := fetchArticlePage(ctx, client, link); err == nil && plain != "" {
			raw = plain
			if hero == "" {
				hero = strings.TrimSpace(og)
			}
			metaKeywords = strings.TrimSpace(kw)
		}
	}
	return raw, hero, metaKeywords, fetchesLeftOut
}

func plainTextRuneLen(htmlOrText string) int {
	t := strings.TrimSpace(spaceSeqRegex.ReplaceAllString(htmlTagRegex.ReplaceAllString(htmlOrText, " "), " "))
	return utf8.RuneCountInString(t)
}

// fetchArticleMeta downloads an article page and extracts og/twitter image and meta keywords (no text extraction).
func fetchArticleMeta(ctx context.Context, client *http.Client, pageURL string) (ogImage string, metaKeywords string, err error) {
	res, err := doScraperGET(ctx, client, pageURL)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		LogHTTPNotOK(pageURL, res.StatusCode)
		return "", "", fmt.Errorf("article page status %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxArticleHTMLBytes))
	if err != nil {
		ScrapeLogf("warn", "", pageURL, "[scraper] GET read body failed url=%s err=%v", pageURL, err)
		return "", "", err
	}
	if scraperguard.ContainsMarkerBytes(raw) {
		ScrapeLogf("info", "", pageURL, "[scraper] skip same-system article meta url=%s", pageURL)
		return "", "", fmt.Errorf("same-system scraper guard marker found")
	}
	doc, err := htmlpkg.Parse(bytes.NewReader(raw))
	if err != nil {
		ScrapeLogf("warn", "", pageURL, "[scraper] GET parse HTML failed url=%s err=%v", pageURL, err)
		return "", "", err
	}
	ogImage, metaKeywords = articleMetaFromDoc(doc)
	ogImage = strings.TrimSpace(html.UnescapeString(ogImage))
	metaKeywords = strings.TrimSpace(html.UnescapeString(metaKeywords))
	return ogImage, metaKeywords, nil
}

// fetchArticlePage downloads an article page, extracts visible text, og:image / twitter:image, and meta keywords when present.
func fetchArticlePage(ctx context.Context, client *http.Client, pageURL string) (plain string, ogImage string, metaKeywords string, err error) {
	_, plain, ogImage, metaKeywords, err = fetchArticlePageDetails(ctx, client, pageURL)
	return plain, ogImage, metaKeywords, err
}

func fetchArticlePageDetails(ctx context.Context, client *http.Client, pageURL string) (title string, plain string, ogImage string, metaKeywords string, err error) {
	reqCtx := ctx
	if reqCtx == nil {
		reqCtx = context.Background()
	}
	perReqCtx, cancel := context.WithTimeout(reqCtx, 25*time.Second)
	defer cancel()
	res, err := doScraperGET(perReqCtx, client, pageURL)
	if err != nil {
		return "", "", "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		LogHTTPNotOK(pageURL, res.StatusCode)
		return "", "", "", "", fmt.Errorf("article page status %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxArticleHTMLBytes))
	if err != nil {
		ScrapeLogf("warn", "", pageURL, "[scraper] GET read body failed url=%s err=%v", pageURL, err)
		return "", "", "", "", err
	}
	if scraperguard.ContainsMarkerBytes(raw) {
		ScrapeLogf("info", "", pageURL, "[scraper] skip same-system article page url=%s", pageURL)
		return "", "", "", "", fmt.Errorf("same-system scraper guard marker found")
	}
	doc, err := htmlpkg.Parse(bytes.NewReader(raw))
	if err != nil {
		ScrapeLogf("warn", "", pageURL, "[scraper] GET parse HTML failed url=%s err=%v", pageURL, err)
		return "", "", "", "", err
	}
	ogImage, metaKeywords = articleMetaFromDoc(doc)
	siteName := articleSiteNameFromDoc(doc)
	h1Title := articleH1TitleFromDoc(doc)
	title = h1Title
	if title == "" {
		title = articleTitleFromDoc(doc)
	}
	if isLikelySiteIdentityTitle(title, pageURL, siteName) && h1Title != "" {
		title = h1Title
	}
	ogImage = strings.TrimSpace(html.UnescapeString(ogImage))
	metaKeywords = strings.TrimSpace(html.UnescapeString(metaKeywords))
	title = strings.TrimSpace(html.UnescapeString(title))
	root := findArticleContentRoot(doc)
	if root == nil {
		root = doc
	}
	out := extractVisibleText(root)
	// Some WordPress themes wrap only the teaser in <article>/<main>.
	// If that happens, fall back to the broader document text when it is
	// materially longer so sitemap-driven scrapes don't end up too short.
	if root != doc && utf8.RuneCountInString(out) < 1200 {
		docText := extractVisibleText(doc)
		if utf8.RuneCountInString(docText) > utf8.RuneCountInString(out)+800 {
			out = docText
		}
	}
	return title, out, ogImage, metaKeywords, nil
}

// articleMetaFromDoc reads og/twitter image and keywords from <meta> tags (first non-empty of each).
func articleMetaFromDoc(n *htmlpkg.Node) (ogImage, metaKeywords string) {
	var walk func(*htmlpkg.Node)
	walk = func(n *htmlpkg.Node) {
		if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "meta") {
			var prop, name, itemprop, content string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "property":
					prop = strings.TrimSpace(a.Val)
				case "name":
					name = strings.TrimSpace(a.Val)
				case "itemprop":
					itemprop = strings.TrimSpace(a.Val)
				case "content":
					content = strings.TrimSpace(a.Val)
				}
			}
			content = strings.TrimSpace(html.UnescapeString(content))
			if ogImage == "" {
				if strings.EqualFold(prop, "og:image") && content != "" {
					ogImage = content
				} else if strings.EqualFold(name, "twitter:image") && content != "" {
					ogImage = content
				} else if strings.EqualFold(name, "twitter:image:src") && content != "" {
					ogImage = content
				}
			}
			if metaKeywords == "" {
				if strings.EqualFold(name, "keywords") && content != "" {
					metaKeywords = content
				} else if strings.EqualFold(name, "news_keywords") && content != "" {
					metaKeywords = content
				} else if strings.EqualFold(itemprop, "keywords") && content != "" {
					metaKeywords = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return ogImage, metaKeywords
}

func articleTitleFromDoc(n *htmlpkg.Node) string {
	var metaTitle string
	var tagTitle string
	var walk func(*htmlpkg.Node)
	walk = func(n *htmlpkg.Node) {
		if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "meta") {
			var prop, name, content string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "property":
					prop = strings.TrimSpace(a.Val)
				case "name":
					name = strings.TrimSpace(a.Val)
				case "content":
					content = strings.TrimSpace(a.Val)
				}
			}
			content = strings.TrimSpace(html.UnescapeString(content))
			if metaTitle == "" {
				if strings.EqualFold(prop, "og:title") && content != "" {
					metaTitle = content
				} else if strings.EqualFold(name, "twitter:title") && content != "" {
					metaTitle = content
				}
			}
		}
		if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "title") && tagTitle == "" {
			tagTitle = strings.TrimSpace(html.UnescapeString(nodeTextContent(n)))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if metaTitle != "" {
		return metaTitle
	}
	return tagTitle
}

func articleSiteNameFromDoc(n *htmlpkg.Node) string {
	var siteName string
	var walk func(*htmlpkg.Node)
	walk = func(n *htmlpkg.Node) {
		if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "meta") {
			var prop, name, content string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "property":
					prop = strings.TrimSpace(a.Val)
				case "name":
					name = strings.TrimSpace(a.Val)
				case "content":
					content = strings.TrimSpace(a.Val)
				}
			}
			content = strings.TrimSpace(html.UnescapeString(content))
			if siteName == "" {
				if strings.EqualFold(prop, "og:site_name") && content != "" {
					siteName = content
				} else if strings.EqualFold(name, "application-name") && content != "" {
					siteName = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return siteName
}

func articleH1TitleFromDoc(n *htmlpkg.Node) string {
	var best string
	var walk func(*htmlpkg.Node)
	walk = func(n *htmlpkg.Node) {
		if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "h1") {
			txt := strings.TrimSpace(html.UnescapeString(nodeTextContent(n)))
			if utf8.RuneCountInString(txt) > utf8.RuneCountInString(best) {
				best = txt
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return best
}

func isLikelySiteIdentityTitle(title, pageURL, siteName string) bool {
	title = strings.TrimSpace(title)
	titleNorm := normalizeTitleIdentityToken(title)
	if titleNorm == "" {
		return true
	}
	siteNorm := normalizeTitleIdentityToken(siteName)
	if siteNorm != "" && titleNorm == siteNorm {
		return true
	}
	hostTokens := titleIdentityHostTokens(pageURL)
	for _, hostTok := range hostTokens {
		if hostTok != "" && titleNorm == hostTok {
			return true
		}
	}
	// Titles like "Blog - Example.com" or "Example.com | Home" are site identity
	// wrappers, not article headlines.
	for _, part := range splitIdentityTitleParts(title) {
		partNorm := normalizeTitleIdentityToken(part)
		if partNorm == "" {
			continue
		}
		if siteNorm != "" && partNorm == siteNorm {
			return true
		}
		for _, hostTok := range hostTokens {
			if hostTok != "" && partNorm == hostTok {
				return true
			}
		}
	}
	return false
}

func splitIdentityTitleParts(title string) []string {
	if title == "" {
		return nil
	}
	clean := strings.NewReplacer("—", "|", "–", "|", "-", "|", "·", "|", "•", "|", "»", "|", ":", "|").Replace(title)
	rawParts := strings.Split(clean, "|")
	parts := make([]string, 0, len(rawParts))
	for _, p := range rawParts {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func titleIdentityHostTokens(pageURL string) []string {
	u, err := url.Parse(strings.TrimSpace(pageURL))
	if err != nil {
		return nil
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return nil
	}
	toks := make([]string, 0, 3)
	toks = append(toks, normalizeTitleIdentityToken(host))
	hostNoWWW := strings.TrimPrefix(host, "www.")
	if hostNoWWW != host {
		toks = append(toks, normalizeTitleIdentityToken(hostNoWWW))
	}
	parts := strings.Split(hostNoWWW, ".")
	if len(parts) >= 2 {
		toks = append(toks, normalizeTitleIdentityToken(parts[len(parts)-2]))
	}
	return toks
}

func normalizeTitleIdentityToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(html.UnescapeString(s)))
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func nodeTextContent(n *htmlpkg.Node) string {
	var sb strings.Builder
	var walk func(*htmlpkg.Node)
	walk = func(n *htmlpkg.Node) {
		if n.Type == htmlpkg.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func resolveAgainstArticleURL(articleURL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	low := strings.ToLower(ref)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return ref
	}
	if strings.HasPrefix(ref, "//") {
		return "https:" + ref
	}
	base, err := url.Parse(articleURL)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(r).String()
}

const maxCollectDepth = 80

func findArticleContentRoot(n *htmlpkg.Node) *htmlpkg.Node {
	best := (*htmlpkg.Node)(nil)
	bestLen := 0

	consider := func(cand *htmlpkg.Node) {
		if cand == nil {
			return
		}
		txt := extractVisibleText(cand)
		l := utf8.RuneCountInString(txt)
		if l > bestLen {
			bestLen = l
			best = cand
		}
	}

	// Prefer semantic containers, but pick the one with the most text.
	consider(firstNodeByTag(n, "article"))
	consider(firstNodeByTag(n, "main"))
	consider(firstNodeByClassHints(n, articleBodyClassHints))

	if best != nil {
		return best
	}
	return nil
}

func firstNodeByTag(n *htmlpkg.Node, tag string) *htmlpkg.Node {
	if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, tag) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r := firstNodeByTag(c, tag); r != nil {
			return r
		}
	}
	return nil
}

func firstNodeByClassHints(n *htmlpkg.Node, hints []string) *htmlpkg.Node {
	var found *htmlpkg.Node
	var walk func(*htmlpkg.Node)
	walk = func(n *htmlpkg.Node) {
		if found != nil {
			return
		}
		if n.Type == htmlpkg.ElementNode {
			for _, a := range n.Attr {
				if a.Key != "class" && a.Key != "id" {
					continue
				}
				val := strings.ToLower(a.Val)
				for _, h := range hints {
					if strings.Contains(val, h) {
						found = n
						return
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return found
}

func collectVisibleText(n *htmlpkg.Node, sb *strings.Builder, depth int) {
	if depth > maxCollectDepth {
		return
	}
	if n.Type == htmlpkg.TextNode {
		t := strings.TrimSpace(n.Data)
		if t != "" {
			sb.WriteString(t)
			sb.WriteString(" ")
		}
		return
	}
	if n.Type != htmlpkg.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectVisibleText(c, sb, depth+1)
		}
		return
	}
	switch strings.ToLower(n.Data) {
	case "script", "style", "noscript", "iframe", "svg", "nav", "form":
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectVisibleText(c, sb, depth+1)
	}
}

func extractVisibleText(n *htmlpkg.Node) string {
	if n == nil {
		return ""
	}
	var sb strings.Builder
	collectVisibleText(n, &sb, 0)
	out := strings.TrimSpace(spaceSeqRegex.ReplaceAllString(sb.String(), " "))
	return html.UnescapeString(out)
}

// StartNewsScraper runs a background worker that imports news posts from configured feeds.
func StartNewsScraper(db *gorm.DB, aiGen *ai.Generator, cfg appconfig.Config) {
	SetScraperLogDB(db)
	go func() {
		for {
			// Run once each cycle; runtime gating is handled per run so
			// category-level enables can still work even when master is off.
			runOnceFromDB(db, aiGen, cfg)

			interval := loadIntervalFromDB(db)
			time.Sleep(interval)
		}
	}()
	ScrapeLogf("info", "", "", "[scraper] started (database-driven settings)")
}

func runOnceFromDB(db *gorm.DB, aiGen *ai.Generator, cfg appconfig.Config) {
	scraperRunMu.Lock()
	defer scraperRunMu.Unlock()

	ok, err := HasRunnableJobs(db)
	if err != nil {
		ScrapeLogf("error", "", "", "[scraper] preflight check failed: %v", err)
		return
	}
	if !ok {
		return
	}
	jobs, err := loadCategoryFeedJobs(db)
	if err != nil {
		ScrapeLogf("error", "", "", "[scraper] load jobs for run budget failed: %v", err)
		return
	}
	pauseMins := 0
	var ss entity.ScraperSetting
	if err := db.Order("id desc").First(&ss).Error; err == nil {
		pauseMins = ss.PauseBetweenCategoriesMins
		if pauseMins < 0 {
			pauseMins = 0
		}
		if pauseMins > 1440 {
			pauseMins = 1440
		}
	}
	pauseTotalMins := 0
	if pauseMins > 0 && len(jobs) > 1 {
		pauseTotalMins = pauseMins * (len(jobs) - 1)
	}
	// Base budget for fetching; add configured inter-category pauses so the context is not cut mid-run.
	runBudget := 15*time.Minute + time.Duration(pauseTotalMins)*time.Minute + 2*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), runBudget)
	defer cancel()
	store := storage.LoadR2ObjectStore(ctx, db, cfg)
	if err := scrapeAllFeeds(ctx, db, aiGen, store); err != nil {
		ScrapeLogf("error", "", "", "[scraper] run failed: %v", err)
	}
}

// RunOnceNow triggers a single scraper cycle immediately.
func RunOnceNow(db *gorm.DB, aiGen *ai.Generator, cfg appconfig.Config) {
	runOnceFromDB(db, aiGen, cfg)
}

// HasRunnableJobs reports whether there is at least one enabled category
// with a non-empty feed URL, after ensuring category rows exist.
func HasRunnableJobs(db *gorm.DB) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database unavailable")
	}
	if err := ensureScraperCategoryFeedRows(db); err != nil {
		return false, err
	}
	if err := backfillLegacyCategoryAuthors(db); err != nil {
		return false, err
	}
	var n int64
	if err := db.Model(&entity.ScraperCategoryFeed{}).
		Where("enabled = ? AND TRIM(feed_urls) <> '' AND TRIM(author_email) <> ''", true).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func loadIntervalFromDB(db *gorm.DB) time.Duration {
	var s entity.ScraperSetting
	if err := db.Order("id desc").First(&s).Error; err != nil {
		return defaultScrapeInterval
	}
	if s.IntervalMins <= 0 {
		return defaultScrapeInterval
	}
	return time.Duration(s.IntervalMins) * time.Minute
}

func scrapeAllFeeds(ctx context.Context, db *gorm.DB, aiGen *ai.Generator, store storage.ObjectStore) error {
	setting, err := loadGlobalRuntimeSetting(db)
	if err != nil {
		return err
	}
	if err := ensureScraperCategoryFeedRows(db); err != nil {
		return err
	}
	if err := backfillLegacyCategoryAuthors(db); err != nil {
		return err
	}
	jobs, err := loadCategoryFeedJobs(db)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		ScrapeLogf("info", "", "", "[scraper] nothing to scrape: no enabled categories with feed URLs (Admin → Scraper settings)")
		return nil
	}
	if ok, reason, err := canRunWithAI(ctx, aiGen, setting); err != nil {
		return err
	} else if !ok {
		ScrapeLogf("info", "", "", "[scraper] skipped: %s", reason)
		return nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	totalCreated := 0
	totalFeeds := 0
	processedFeeds := 0
	failedFeeds := 0
	// Prevent over-scraping the same domain within a single run, but do it per-category
	// so one domain can still contribute to multiple categories.
	createdByDomain := make(map[string]int)
	for _, job := range jobs {
		totalFeeds += len(job.FeedURLs)
	}

	ScrapeLogf("info", "", "", "[scraper] run start jobs=%d total_feeds=%d", len(jobs), totalFeeds)
	for ji, job := range jobs {
		if !isScraperEnabled(db) {
			ScrapeLogf("warn", "", "", "[scraper] stopped mid-run: disabled")
			return nil
		}
		authorID, err := resolveScraperAuthorByEmail(db, job.AuthorEmail)
		if err != nil {
			ScrapeLogf("warn", job.CategorySlug, "", "[scraper] skip category slug=%q: %v", job.CategorySlug, err)
			continue
		}
		categoryID, err := resolveCategoryID(db, job.CategorySlug)
		if err != nil {
			ScrapeLogf("warn", job.CategorySlug, "", "[scraper] skip category slug=%q err=%v", job.CategorySlug, err)
			continue
		}
		ranFeeds := false
		for _, rawFeedURL := range job.FeedURLs {
			if !isScraperEnabled(db) {
				ScrapeLogf("warn", "", "", "[scraper] stopped mid-run: disabled")
				return nil
			}
			ranFeeds = true
			processedFeeds++
			created, feedErr := scrapeSingleFeed(
				ctx,
				client,
				db,
				aiGen,
				store,
				setting,
				createdByDomain,
				job.CategorySlug,
				job.IncludeSource,
				job.ArticleStatus,
				job.ImageMode,
				job.WebSearchSource,
				rawFeedURL,
				authorID,
				categoryID,
				1, // Limit each feed URL to at most one new post per interval.
			)
			if feedErr != nil {
				failedFeeds++
				ScrapeLogf("error", job.CategorySlug, rawFeedURL, "[scraper] feed error category=%s url=%s err=%v", job.CategorySlug, rawFeedURL, feedErr)
				continue
			}
			totalCreated += created
		}
		if ranFeeds && ji < len(jobs)-1 && setting.PauseBetweenCategoriesMins > 0 {
			ScrapeLogf("info", job.CategorySlug, "", "[scraper] inter-category pause %d min before next category", setting.PauseBetweenCategoriesMins)
			if err := sleepBetweenCategories(ctx, db, setting.PauseBetweenCategoriesMins); err != nil {
				return err
			}
		}
	}
	ScrapeLogf(
		"info",
		"",
		"",
		"[scraper] completed jobs=%d total_feeds=%d processed_feeds=%d failed_feeds=%d new_posts=%d",
		len(jobs),
		totalFeeds,
		processedFeeds,
		failedFeeds,
		totalCreated,
	)
	return nil
}

func canRunWithAI(ctx context.Context, aiGen *ai.Generator, setting *runtimeSetting) (bool, string, error) {
	if setting == nil {
		return false, "missing scraper setting", nil
	}
	provider := strings.ToLower(strings.TrimSpace(setting.ScraperAIProvider))
	if provider == "" || provider == "off" {
		return false, "AI provider is disabled", nil
	}
	if aiGen == nil {
		return false, fmt.Sprintf("AI generator unavailable for provider=%s", provider), nil
	}
	if provider == "auto" {
		hasAny, err := aiGen.HasAnyKey(ctx)
		if err != nil {
			return false, "", err
		}
		if !hasAny {
			return false, "AI provider=auto but no API key configured", nil
		}
		return true, "", nil
	}

	ok, err := aiGen.HasProviderKey(ctx, ai.Provider(provider))
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, fmt.Sprintf("AI API key not configured for provider=%s", provider), nil
	}
	return true, "", nil
}

type runtimeSetting struct {
	Enabled                    bool
	PauseBetweenCategoriesMins int
	ScraperAIProvider          string
	ScraperPromptTitle         string
	ScraperPromptContent       string
}

type categoryFeedJob struct {
	CategorySlug  string
	FeedURLs      []string
	IncludeSource bool
	ArticleStatus string // "draft" | "published"
	ImageMode         string // "scrape" | "generate" (template) | "generate_image" (AI) | "web_search"
	WebSearchSource   string // when ImageMode is web_search: auto, pexels, pixabay, …
	AuthorEmail       string
}

func sleepBetweenCategories(ctx context.Context, db *gorm.DB, mins int) error {
	if mins <= 0 {
		return nil
	}
	if mins > 1440 {
		mins = 1440
	}
	deadline := time.Now().Add(time.Duration(mins) * time.Minute)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if !isScraperEnabled(db) {
				ScrapeLogf("warn", "", "", "[scraper] stopped during inter-category pause: disabled")
				return nil
			}
		}
	}
	return nil
}

// rewriteLegacyFeedURL maps feed URLs that no longer resolve (e.g. rss.cnnindonesia.com) to the current host.
func rewriteLegacyFeedURL(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	if !strings.EqualFold(u.Host, "rss.cnnindonesia.com") {
		return raw
	}
	segs := strings.Trim(u.Path, "/")
	if segs == "" {
		return "https://www.cnnindonesia.com/nasional/rss"
	}
	if strings.HasSuffix(strings.ToLower(segs), "/rss") {
		return "https://www.cnnindonesia.com/" + segs
	}
	return "https://www.cnnindonesia.com/" + segs + "/rss"
}

func loadGlobalRuntimeSetting(db *gorm.DB) (*runtimeSetting, error) {
	var s entity.ScraperSetting
	if err := db.Order("id desc").First(&s).Error; err != nil {
		return nil, fmt.Errorf("scraper setting not found: %w", err)
	}
	var aiSetting entity.AISetting
	_ = db.First(&aiSetting).Error

	provider := strings.TrimSpace(aiSetting.ScraperAIProvider)
	if provider == "" {
		provider = "auto"
	}

	pauseBetween := s.PauseBetweenCategoriesMins
	if pauseBetween < 0 {
		pauseBetween = 0
	}
	if pauseBetween > 1440 {
		pauseBetween = 1440
	}
	return &runtimeSetting{
		Enabled:                    s.Enabled,
		PauseBetweenCategoriesMins: pauseBetween,
		ScraperAIProvider:          provider,
		ScraperPromptTitle:         strings.TrimSpace(aiSetting.ScraperPromptTitle),
		ScraperPromptContent:       strings.TrimSpace(aiSetting.ScraperPromptContent),
	}, nil
}

// backfillLegacyCategoryAuthors copies scraper_settings.author_email into per-category rows
// that still lack author_email (older installs).
func backfillLegacyCategoryAuthors(db *gorm.DB) error {
	var global entity.ScraperSetting
	if err := db.Order("id desc").First(&global).Error; err != nil {
		return nil
	}
	leg := strings.TrimSpace(global.AuthorEmail)
	if leg == "" {
		return nil
	}
	return db.Model(&entity.ScraperCategoryFeed{}).
		Where("author_email IS NULL OR TRIM(author_email) = ?", "").
		Update("author_email", leg).Error
}

func ensureScraperCategoryFeedRows(db *gorm.DB) error {
	if err := repositories.NewScraperCategoryFeedRepository(db).DedupeByCanonicalSlug(); err != nil {
		return err
	}
	var cats []entity.Category
	if err := db.Find(&cats).Error; err != nil {
		return err
	}
	if len(cats) == 0 {
		return db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&entity.ScraperCategoryFeed{}).Error
	}
	slugs := make([]string, 0, len(cats))
	for _, c := range cats {
		slugs = append(slugs, strings.TrimSpace(c.Slug))
	}
	if err := db.Where("category_slug NOT IN ?", slugs).Delete(&entity.ScraperCategoryFeed{}).Error; err != nil {
		return err
	}
	var rows []entity.ScraperCategoryFeed
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	legacyAuthor := ""
	var global entity.ScraperSetting
	if err := db.Order("id desc").First(&global).Error; err == nil {
		legacyAuthor = strings.TrimSpace(global.AuthorEmail)
	}
	have := make(map[string]bool, len(rows))
	for _, r := range rows {
		have[entity.CanonicalCategorySlug(r.CategorySlug)] = true
	}
	var missing []entity.ScraperCategoryFeed
	for _, c := range cats {
		catSlug := strings.TrimSpace(c.Slug)
		if !have[entity.CanonicalCategorySlug(catSlug)] {
			missing = append(missing, entity.ScraperCategoryFeed{
				CategorySlug:  catSlug,
				FeedURLs:      "",
				Enabled:       true,
				IncludeSource: true,
				ArticleStatus: "published",
				ImageMode:         "scrape",
				WebSearchSource:   "auto",
				AutoTag:           true,
				AuthorEmail:   legacyAuthor,
			})
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return db.Create(&missing).Error
}

func splitScraperFeedURLs(raw string) []string {
	feeds := make([]string, 0)
	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n", ";", ",", "\n", ",").Replace(raw)
	for _, part := range strings.Split(normalized, ",") {
		part = rewriteLegacyFeedURL(strings.TrimSpace(part))
		if part != "" {
			feeds = append(feeds, part)
		}
	}
	return feeds
}

func matchWordPressPostSitemapPattern(raw string) (prefix string, start int, suffix string, ok bool) {
	m := wpPostSitemapRE.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) != 4 {
		return "", 0, "", false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil || n < 1 {
		return "", 0, "", false
	}
	return m[1], n, m[3], true
}

func fetchSitemapArticleURLs(ctx context.Context, client *http.Client, sitemapURL string) ([]string, int, error) {
	reqCtx := ctx
	if reqCtx == nil {
		reqCtx = context.Background()
	}
	perReqCtx, cancel := context.WithTimeout(reqCtx, 60*time.Second)
	defer cancel()
	res, err := doScraperGET(perReqCtx, client, sitemapURL)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	status := res.StatusCode
	if status < 200 || status > 299 {
		LogHTTPNotOK(sitemapURL, status)
		return nil, status, fmt.Errorf("sitemap status %d", status)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxSitemapBytes))
	if err != nil {
		ScrapeLogf("warn", "", sitemapURL, "[scraper] GET sitemap read body failed url=%s err=%v", sitemapURL, err)
		return nil, status, err
	}
	safe := sanitizeXMLAmpersandsPreservingCDATA(string(raw))
	var doc sitemapURLSet
	dec := xml.NewDecoder(strings.NewReader(safe))
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil {
		ScrapeLogf("warn", "", sitemapURL, "[scraper] GET sitemap XML decode failed url=%s err=%v", sitemapURL, err)
		return nil, status, err
	}
	out := make([]string, 0, len(doc.URLs))
	for _, u := range doc.URLs {
		loc := strings.TrimSpace(u.Loc)
		if strings.HasPrefix(loc, "http://") || strings.HasPrefix(loc, "https://") {
			out = append(out, loc)
		}
	}
	return out, status, nil
}

func scrapeWordPressSequentialSitemaps(
	ctx context.Context,
	client *http.Client,
	db *gorm.DB,
	aiGen *ai.Generator,
	store storage.ObjectStore,
	setting *runtimeSetting,
	createdByDomain map[string]int,
	categorySlug string,
	includeSource bool,
	articleStatus string,
	imageMode string,
	webSearchSource string,
	rawSitemapURL string,
	authorID uint,
	categoryID uint,
	maxNewPosts int,
) (int, error) {
	prefix, start, suffix, ok := matchWordPressPostSitemapPattern(rawSitemapURL)
	if !ok {
		return 0, fmt.Errorf("invalid wordpress sitemap pattern")
	}
	totalCreated := 0
	consecutiveMisses := 0
	const maxSequentialSitemaps = 100
	for i := start; i < start+maxSequentialSitemaps; i++ {
		if !isScraperEnabled(db) {
			return totalCreated, nil
		}
		sitemapURL := prefix + strconv.Itoa(i) + suffix
		articleURLs, status, err := fetchSitemapArticleURLs(ctx, client, sitemapURL)
		if err != nil {
			if status == http.StatusNotFound || status == http.StatusGone || status == http.StatusForbidden {
				break
			}
			consecutiveMisses++
			if consecutiveMisses >= 2 {
				break
			}
			continue
		}
		if len(articleURLs) == 0 {
			break
		}
		consecutiveMisses = 0
		for _, articleURL := range articleURLs {
			if !isScraperEnabled(db) {
				return totalCreated, nil
			}
			if !shouldScrapeArticleURL(articleURL) {
				continue
			}
			if maxNewPosts > 0 && totalCreated >= maxNewPosts {
				return totalCreated, nil
			}
			domain := domainForScrapedLink(articleURL, sitemapURL)
			key := ""
			if domain != "" {
				key = strings.TrimSpace(categorySlug) + "|" + domain
			}
			if key != "" && createdByDomain != nil && createdByDomain[key] >= maxCreatedPostsPerDomainPerCategoryPerRun {
				continue
			}
			title, plain, hero, kw, err := fetchArticlePageDetails(ctx, client, articleURL)
			if err != nil {
				ScrapeLogf("warn", categorySlug, rawSitemapURL, "[scraper] sitemap article error url=%s err=%v", articleURL, err)
				continue
			}
			ok, err := createPostIfNew(ctx, client, db, aiGen, store, setting, postCandidate{
				Title:        title,
				Link:         articleURL,
				Description:  plain,
				HeroImageURL: hero,
				MetaKeywords: kw,
				CategorySlug: categorySlug,
				FeedURL:      rawSitemapURL,
			}, authorID, categoryID, includeSource, articleStatus, imageMode, webSearchSource)
			if err != nil {
				ScrapeLogf("warn", categorySlug, rawSitemapURL, "[scraper] sitemap create error title=%q url=%s err=%v", title, articleURL, err)
				continue
			}
			if ok {
				totalCreated++
				if key != "" && createdByDomain != nil {
					createdByDomain[key]++
				}
			}
		}
	}
	return totalCreated, nil
}

func loadCategoryFeedJobs(db *gorm.DB) ([]categoryFeedJob, error) {
	var rows []entity.ScraperCategoryFeed
	if err := db.Order("category_slug asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	jobs := make([]categoryFeedJob, 0)
	for _, r := range rows {
		if !r.Enabled {
			continue
		}
		feeds := splitScraperFeedURLs(r.FeedURLs)
		if len(feeds) == 0 {
			continue
		}
		slug := strings.TrimSpace(r.CategorySlug)
		if slug == "" {
			continue
		}
		imgMode := strings.TrimSpace(r.ImageMode)
		if imgMode == "" {
			imgMode = "scrape"
		}
		authorEm := strings.TrimSpace(r.AuthorEmail)
		if authorEm == "" {
			ScrapeLogf("warn", slug, "", "[scraper] skip category slug=%q: author email not set (Admin → Scraper settings)", slug)
			continue
		}
		webSrc := strings.TrimSpace(r.WebSearchSource)
		if webSrc == "" {
			webSrc = "auto"
		}
		jobs = append(jobs, categoryFeedJob{
			CategorySlug:    slug,
			FeedURLs:        feeds,
			IncludeSource:   r.IncludeSource,
			ArticleStatus:   strings.ToLower(strings.TrimSpace(r.ArticleStatus)),
			ImageMode:       imgMode,
			WebSearchSource: NormalizeWebSearchSource(webSrc),
			AuthorEmail:     authorEm,
		})
	}
	return jobs, nil
}

func scrapeSingleFeed(
	ctx context.Context,
	client *http.Client,
	db *gorm.DB,
	aiGen *ai.Generator,
	store storage.ObjectStore,
	setting *runtimeSetting,
	createdByDomain map[string]int,
	categorySlug string,
	includeSource bool,
	articleStatus string,
	imageMode string,
	webSearchSource string,
	rawFeedURL string,
	authorID uint,
	categoryID uint,
	maxNewPosts int,
) (int, error) {
	if _, _, _, ok := matchWordPressPostSitemapPattern(rawFeedURL); ok {
		return scrapeWordPressSequentialSitemaps(
			ctx,
			client,
			db,
			aiGen,
			store,
			setting,
			createdByDomain,
			categorySlug,
			includeSource,
			articleStatus,
			imageMode,
			webSearchSource,
			rawFeedURL,
			authorID,
			categoryID,
			maxNewPosts,
		)
	}
	body, err := fetchFeedBytes(ctx, client, rawFeedURL, 0)
	if err != nil {
		return 0, err
	}
	if scraperguard.ContainsMarkerBytes(body) {
		ScrapeLogf("info", categorySlug, rawFeedURL, "[scraper] skip same-system feed url=%s", rawFeedURL)
		return 0, nil
	}
	safe := sanitizeXMLAmpersandsPreservingCDATA(string(body))
	var doc rssDocument
	dec := xml.NewDecoder(strings.NewReader(safe))
	dec.Strict = false
	if err := dec.Decode(&doc); err != nil {
		return 0, err
	}
	if len(doc.Channel.Items) == 0 && len(doc.Entries) == 0 {
		var sitemapDoc sitemapURLSet
		sdec := xml.NewDecoder(strings.NewReader(safe))
		sdec.Strict = false
		if err := sdec.Decode(&sitemapDoc); err == nil && len(sitemapDoc.URLs) > 0 {
			return scrapeSitemapURLSet(
				ctx,
				client,
				db,
				aiGen,
				store,
				setting,
				createdByDomain,
				categorySlug,
				includeSource,
				articleStatus,
				imageMode,
				webSearchSource,
				rawFeedURL,
				authorID,
				categoryID,
				maxNewPosts,
				sitemapDoc.URLs,
			)
		}
		var sitemapIdx sitemapIndex
		idec := xml.NewDecoder(strings.NewReader(safe))
		idec.Strict = false
		if err := idec.Decode(&sitemapIdx); err == nil && len(sitemapIdx.Sitemaps) > 0 {
			return scrapeSitemapIndex(
				ctx,
				client,
				db,
				aiGen,
				store,
				setting,
				createdByDomain,
				categorySlug,
				includeSource,
				articleStatus,
				imageMode,
				webSearchSource,
				rawFeedURL,
				authorID,
				categoryID,
				maxNewPosts,
				sitemapIdx.Sitemaps,
			)
		}
	}

	created := 0
	articleFetchesLeft := maxArticlePageFetchesPerFeed
	for _, item := range doc.Channel.Items {
		if !isScraperEnabled(db) {
			return created, nil
		}
		if maxNewPosts > 0 && created >= maxNewPosts {
			return created, nil
		}
		link := rssItemLink(item)
		if !shouldScrapeArticleURL(link) {
			continue
		}
		domain := domainForScrapedLink(link, rawFeedURL)
		key := ""
		if domain != "" {
			key = strings.TrimSpace(categorySlug) + "|" + domain
		}
		if key != "" && createdByDomain != nil && createdByDomain[key] >= maxCreatedPostsPerDomainPerCategoryPerRun {
			continue
		}
		var rawBody string
		var hero string
		var pageMetaKw string
		rawBody, hero, pageMetaKw, articleFetchesLeft = rssItemBodyHTML(ctx, client, item, link, articleFetchesLeft)
		if scraperguard.ContainsMarkerString(rawBody) {
			ScrapeLogf("info", categorySlug, rawFeedURL, "[scraper] skip same-system RSS item url=%s", link)
			continue
		}
		ok, err := createPostIfNew(ctx, client, db, aiGen, store, setting, postCandidate{
			Title:        item.Title,
			Link:         link,
			Description:  rawBody,
			HeroImageURL: hero,
			MetaKeywords: pageMetaKw,
			CategorySlug: categorySlug,
			FeedURL:      rawFeedURL,
		}, authorID, categoryID, includeSource, articleStatus, imageMode, webSearchSource)
		if err != nil {
			ScrapeLogf("warn", categorySlug, rawFeedURL, "[scraper] item error title=%q err=%v", item.Title, err)
			continue
		}
		if ok {
			created++
			if key != "" && createdByDomain != nil {
				createdByDomain[key]++
			}
		}
	}

	articleFetchesLeft = maxArticlePageFetchesPerFeed
	for _, entry := range doc.Entries {
		if !isScraperEnabled(db) {
			return created, nil
		}
		if maxNewPosts > 0 && created >= maxNewPosts {
			return created, nil
		}
		link := atomEntryLink(entry)
		if !shouldScrapeArticleURL(link) {
			continue
		}
		domain := domainForScrapedLink(link, rawFeedURL)
		key := ""
		if domain != "" {
			key = strings.TrimSpace(categorySlug) + "|" + domain
		}
		if key != "" && createdByDomain != nil && createdByDomain[key] >= maxCreatedPostsPerDomainPerCategoryPerRun {
			continue
		}
		raw := firstNonEmpty(entry.Content, entry.Summary)
		if scraperguard.ContainsMarkerString(raw) {
			ScrapeLogf("info", categorySlug, rawFeedURL, "[scraper] skip same-system Atom entry url=%s", link)
			continue
		}
		hero := strings.TrimSpace(postmeta.FirstImageURL(raw))
		pageMetaKw := ""

		// Like RSS: if the feed doesn't include an image, try og/twitter meta first.
		if hero == "" && articleFetchesLeft > 0 && strings.HasPrefix(link, "http") {
			articleFetchesLeft--
			if og, kw, err := fetchArticleMeta(ctx, client, link); err == nil {
				if hero == "" {
					hero = strings.TrimSpace(og)
				}
				pageMetaKw = strings.TrimSpace(kw)
			}
		}

		if articleFetchesLeft > 0 && plainTextRuneLen(raw) < minPlainCharsBeforeArticleFetch && strings.HasPrefix(link, "http") {
			articleFetchesLeft--
			if plain, og, kw, err := fetchArticlePage(ctx, client, link); err == nil && plain != "" {
				raw = plain
				if hero == "" {
					hero = strings.TrimSpace(og)
				}
				pageMetaKw = strings.TrimSpace(kw)
			}
		}
		ok, err := createPostIfNew(ctx, client, db, aiGen, store, setting, postCandidate{
			Title:        entry.Title,
			Link:         link,
			Description:  raw,
			HeroImageURL: hero,
			MetaKeywords: pageMetaKw,
			CategorySlug: categorySlug,
			FeedURL:      rawFeedURL,
		}, authorID, categoryID, includeSource, articleStatus, imageMode, webSearchSource)
		if err != nil {
			ScrapeLogf("warn", categorySlug, rawFeedURL, "[scraper] entry error title=%q err=%v", entry.Title, err)
			continue
		}
		if ok {
			created++
			if key != "" && createdByDomain != nil {
				createdByDomain[key]++
			}
		}
	}

	return created, nil
}

func scrapeSitemapIndex(
	ctx context.Context,
	client *http.Client,
	db *gorm.DB,
	aiGen *ai.Generator,
	store storage.ObjectStore,
	setting *runtimeSetting,
	createdByDomain map[string]int,
	categorySlug string,
	includeSource bool,
	articleStatus string,
	imageMode string,
	webSearchSource string,
	rawFeedURL string,
	authorID uint,
	categoryID uint,
	maxNewPosts int,
	sitemaps []sitemapRef,
) (int, error) {
	created := 0
	for _, sm := range sitemaps {
		if !isScraperEnabled(db) {
			return created, nil
		}
		if maxNewPosts > 0 && created >= maxNewPosts {
			return created, nil
		}
		childURL := strings.TrimSpace(sm.Loc)
		if childURL == "" {
			continue
		}
		articleURLs, _, err := fetchSitemapArticleURLs(ctx, client, childURL)
		if err != nil {
			ScrapeLogf("warn", categorySlug, rawFeedURL, "[scraper] sitemap index child error url=%s err=%v", childURL, err)
			continue
		}
		urlsetRows := make([]sitemapURL, 0, len(articleURLs))
		for _, u := range articleURLs {
			urlsetRows = append(urlsetRows, sitemapURL{Loc: u})
		}
		n, err := scrapeSitemapURLSet(
			ctx,
			client,
			db,
			aiGen,
			store,
			setting,
			createdByDomain,
			categorySlug,
			includeSource,
			articleStatus,
			imageMode,
			webSearchSource,
			rawFeedURL,
			authorID,
			categoryID,
			maxNewPosts-created,
			urlsetRows,
		)
		if err != nil {
			ScrapeLogf("warn", categorySlug, rawFeedURL, "[scraper] sitemap index child scrape error url=%s err=%v", childURL, err)
			continue
		}
		created += n
	}
	return created, nil
}

func scrapeSitemapURLSet(
	ctx context.Context,
	client *http.Client,
	db *gorm.DB,
	aiGen *ai.Generator,
	store storage.ObjectStore,
	setting *runtimeSetting,
	createdByDomain map[string]int,
	categorySlug string,
	includeSource bool,
	articleStatus string,
	imageMode string,
	webSearchSource string,
	rawFeedURL string,
	authorID uint,
	categoryID uint,
	maxNewPosts int,
	urls []sitemapURL,
) (int, error) {
	created := 0
	for _, u := range urls {
		if !isScraperEnabled(db) {
			return created, nil
		}
		if maxNewPosts > 0 && created >= maxNewPosts {
			return created, nil
		}
		articleURL := strings.TrimSpace(u.Loc)
		if !shouldScrapeArticleURL(articleURL) {
			continue
		}
		domain := domainForScrapedLink(articleURL, rawFeedURL)
		key := ""
		if domain != "" {
			key = strings.TrimSpace(categorySlug) + "|" + domain
		}
		if key != "" && createdByDomain != nil && createdByDomain[key] >= maxCreatedPostsPerDomainPerCategoryPerRun {
			continue
		}
		title, plain, hero, kw, err := fetchArticlePageDetails(ctx, client, articleURL)
		if err != nil {
			ScrapeLogf("warn", categorySlug, rawFeedURL, "[scraper] sitemap urlset article error url=%s err=%v", articleURL, err)
			continue
		}
		ok, err := createPostIfNew(ctx, client, db, aiGen, store, setting, postCandidate{
			Title:        title,
			Link:         articleURL,
			Description:  plain,
			HeroImageURL: hero,
			MetaKeywords: kw,
			CategorySlug: categorySlug,
			FeedURL:      rawFeedURL,
		}, authorID, categoryID, includeSource, articleStatus, imageMode, webSearchSource)
		if err != nil {
			ScrapeLogf("warn", categorySlug, rawFeedURL, "[scraper] sitemap urlset create error title=%q url=%s err=%v", title, articleURL, err)
			continue
		}
		if ok {
			created++
			if key != "" && createdByDomain != nil {
				createdByDomain[key]++
			}
		}
	}
	return created, nil
}

func isScraperEnabled(db *gorm.DB) bool {
	var s entity.ScraperSetting
	if err := db.Select("enabled").Order("id desc").First(&s).Error; err != nil {
		// Fail-open to avoid accidental scraper shutdown on transient DB read issues.
		return true
	}
	if s.Enabled {
		return true
	}

	var n int64
	if err := db.Model(&entity.ScraperCategoryFeed{}).
		Where("enabled = ? AND TRIM(feed_urls) <> ''", true).
		Count(&n).Error; err != nil {
		// Fail-open to avoid accidental scraper shutdown on transient DB read issues.
		return true
	}
	return n > 0
}

func domainForScrapedLink(link string, fallbackURL string) string {
	link = strings.TrimSpace(link)
	if link != "" {
		if u, err := url.Parse(link); err == nil && u != nil && u.Host != "" {
			return strings.ToLower(u.Host)
		}
	}
	fallbackURL = strings.TrimSpace(fallbackURL)
	if fallbackURL != "" {
		if u, err := url.Parse(fallbackURL); err == nil && u != nil && u.Host != "" {
			return strings.ToLower(u.Host)
		}
	}
	return ""
}

func shouldScrapeArticleURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	seg := strings.Trim(strings.TrimSpace(u.Path), "/")
	if seg == "" {
		return false
	}
	parts := strings.Split(seg, "/")
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return false
	}
	// Article slug heuristic: require at least 2 hyphens
	// (e.g. /ini-judul-artikel).
	return strings.Count(last, "-") >= 2
}

type postCandidate struct {
	Title        string
	Link         string
	Description  string
	HeroImageURL string
	MetaKeywords string
	// CategorySlug and FeedURL are optional; used only for AI paraphrase audit logs.
	CategorySlug string
	FeedURL      string
}

func resolveScrapedPostImage(ctx context.Context, client *http.Client, store storage.ObjectStore, in postCandidate, articleURL, postSlug string, storeMaxLongestEdge, storeWebPQuality int) *string {
	candidates := make([]string, 0, 8)
	if h := strings.TrimSpace(in.HeroImageURL); h != "" {
		candidates = append(candidates, h)
	}
	for _, u := range allImageURLsFromHTML(in.Description) {
		candidates = append(candidates, u)
	}
	if len(candidates) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	var firstLocal *string
	for _, rawURL := range candidates {
		u := strings.TrimSpace(rawURL)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}

		abs := resolveAgainstArticleURL(articleURL, u)
		if abs == "" {
			continue
		}
		if local, err := downloadRemotePostImage(ctx, client, store, abs, articleURL, postSlug, storeMaxLongestEdge, storeWebPQuality); err == nil && local != nil {
			return local
		}
	}
	return firstLocal
}

func allImageURLsFromHTML(htmlFrag string) []string {
	if strings.TrimSpace(htmlFrag) == "" {
		return nil
	}
	matches := imgSrcRegex.FindAllStringSubmatch(htmlFrag, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		src := strings.TrimSpace(html.UnescapeString(m[1]))
		if src == "" {
			continue
		}
		out = append(out, src)
	}
	return out
}

const DefaultScrapeTitlePrompt = "Parafrase judul berita berikut ke Bahasa Indonesia yang natural.\n- Pertahankan makna, jangan tambah fakta\n- Jangan pakai tanda kutip\n- Output: judul saja (1 baris)\n\nJudul:\n{{title}}"

// DefaultScrapeBodyPrompt is the default AI instruction for paraphrasing scraped article body into HTML (also seeded into ai_setting).
const DefaultScrapeBodyPrompt = `Tugas:
Parafrase artikel berita berikut ke dalam Bahasa Indonesia yang natural, jelas, dan enak dibaca.

Aturan:
- Pertahankan makna asli, jangan menambah atau mengurangi fakta.
- Gunakan gaya bahasa jurnalistik yang rapi dan profesional.
- Susun ulang kalimat agar tidak terdeteksi sebagai duplikat, tetapi tetap setia pada isi.
- Pertahankan panjang konten agar mendekati naskah asli (target 90-110% dari jumlah kata input).
- Hindari ringkasan berlebihan; jangan memangkas detail penting.
- WAJIB ikuti format ini untuk setiap paragraf:
  - Hasil parafrase konten maksimal sampai titik kedua per paragraf.
- Artinya: setiap paragraf hanya boleh berisi paling banyak 2 kalimat yang diakhiri tanda titik (.).
- Jika paragraf hasil parafrase melebihi 2 kalimat, potong dan berhenti di titik kedua.
- Jangan buat paragraf yang berisi 3 kalimat atau lebih.

Format Output:
- Output HARUS berupa HTML murni (tanpa markdown, tanpa fenced code / blok kode).
- Output HARUS diawali dengan paragraf pembuka menggunakan <p>.
- Setelah minimal 1 paragraf pembuka, boleh gunakan <h2> atau <h3> untuk subjudul jika diperlukan.
- Gunakan hanya tag berikut:
  <h2>, <h3>, <p>, <ul>, <ol>, <li>, <strong>, <em>, <a>, <blockquote>, <table>, <thead>, <tbody>, <tr>, <th>, <td>

Aturan Tambahan:
- Jangan gunakan CSS, atribut style, class, id, atau tag <script>.
- Jangan menambahkan komentar atau teks di luar HTML.
- Jika terdapat data numerik seperti perbandingan, statistik, atau daftar angka, sajikan dalam tabel HTML dengan header yang jelas.
- Jika tidak ada data numerik, jangan buat tabel.

Penanganan Error:
- Jika konten kosong, tidak valid, atau terlalu pendek, tetap hasilkan:
  <p>Konten tidak tersedia.</p>

Isi Artikel:
{{content}}`

func applyPromptTemplate(tpl, title, content string) string {
	out := strings.ReplaceAll(tpl, "{{title}}", title)
	out = strings.ReplaceAll(out, "{{content}}", content)
	return out
}

// scraperAIGenerate runs one AI completion; used is the backend that served the request (e.g. gemini when setting is auto).
func scraperAIGenerate(ctx context.Context, aiGen *ai.Generator, providerSetting string, prompt string) (text string, used string, err error) {
	p := strings.ToLower(strings.TrimSpace(providerSetting))
	if p == "auto" {
		out, prov, err := aiGen.GenerateBlogContentWithProvider(ctx, prompt)
		return out, string(prov), err
	}
	out, err := aiGen.GenerateWith(ctx, ai.Provider(p), prompt)
	return out, p, err
}

func paraphraseScrapedTitleAndBody(ctx context.Context, aiGen *ai.Generator, setting *runtimeSetting, title string, plainBody string, categorySlug string, articleURL string, feedURL string) (string, string, error) {
	if aiGen == nil {
		return title, plainBody, nil
	}
	if setting == nil {
		return title, plainBody, nil
	}
	p := strings.ToLower(strings.TrimSpace(setting.ScraperAIProvider))
	if p == "" || p == "off" {
		return title, plainBody, nil
	}
	title = strings.TrimSpace(title)
	plainBody = strings.TrimSpace(plainBody)
	if title == "" || plainBody == "" {
		return title, plainBody, nil
	}

	const maxBodyRunes = 12000
	if utf8.RuneCountInString(plainBody) > maxBodyRunes {
		plainBody = string([]rune(plainBody)[:maxBodyRunes])
	}

	tplTitle := strings.TrimSpace(setting.ScraperPromptTitle)
	if tplTitle == "" {
		tplTitle = DefaultScrapeTitlePrompt
	}
	tplBody := strings.TrimSpace(setting.ScraperPromptContent)
	if tplBody == "" {
		tplBody = DefaultScrapeBodyPrompt
	}
	titlePrompt := applyPromptTemplate(tplTitle, title, plainBody)
	bodyPrompt := applyPromptTemplate(tplBody, title, plainBody)

	aiCtx := ctx
	if aiCtx == nil {
		aiCtx = context.Background()
	}
	logURL := strings.TrimSpace(articleURL)
	if logURL == "" {
		logURL = strings.TrimSpace(feedURL)
	}
	ScrapeLogWithProviders("info", categorySlug, logURL, "", "[scraper] AI API call start phase=title feed_cfg=%s title_preview=%.120q", strings.TrimSpace(feedURL), title)
	t0 := time.Now()
	perCallCtx, cancel := context.WithTimeout(aiCtx, 75*time.Second)
	newTitle, usedTitle, err := scraperAIGenerate(perCallCtx, aiGen, p, titlePrompt)
	cancel()
	if err != nil {
		ScrapeLogWithProviders("warn", categorySlug, logURL, usedTitle, "[scraper] AI API call failed phase=title duration_ms=%d err=%v", time.Since(t0).Milliseconds(), err)
		return title, plainBody, err
	}
	ScrapeLogWithProviders("info", categorySlug, logURL, usedTitle, "[scraper] AI API call ok phase=title duration_ms=%d out_runes=%d", time.Since(t0).Milliseconds(), utf8.RuneCountInString(strings.TrimSpace(newTitle)))
	var newBody string
	ScrapeLogWithProviders("info", categorySlug, logURL, "", "[scraper] AI API call start phase=body source_words=%d", countWordsLoose(plainBody))
	t1 := time.Now()
	perCallCtx, cancel = context.WithTimeout(aiCtx, 75*time.Second)
	newBody, usedBody, err := scraperAIGenerate(perCallCtx, aiGen, p, bodyPrompt)
	cancel()
	if err != nil {
		ScrapeLogWithProviders("warn", categorySlug, logURL, usedBody, "[scraper] AI API call failed phase=body duration_ms=%d err=%v", time.Since(t1).Milliseconds(), err)
		return strings.TrimSpace(newTitle), plainBody, err
	}
	newBody = strings.TrimSpace(newBody)
	ScrapeLogWithProviders("info", categorySlug, logURL, usedBody, "[scraper] AI API call ok phase=body duration_ms=%d out_words=%d", time.Since(t1).Milliseconds(), countWordsLoose(newBody))
	if isParaphraseTooShort(plainBody, newBody) {
		retryPrompt := bodyPrompt + "\n\nInstruksi tambahan penting:\n- Panjang output WAJIB mendekati input (minimal 85% dari jumlah kata input).\n- Jangan meringkas; pertahankan detail, nama, angka, kutipan, konteks, dan urutan informasi utama."
		ScrapeLogWithProviders("info", categorySlug, logURL, "", "[scraper] AI API call start phase=body_retry_short")
		t2 := time.Now()
		perCallCtx, cancel = context.WithTimeout(aiCtx, 75*time.Second)
		newBody, usedBody, err = scraperAIGenerate(perCallCtx, aiGen, p, retryPrompt)
		cancel()
		if err != nil {
			ScrapeLogWithProviders("warn", categorySlug, logURL, usedBody, "[scraper] AI API call failed phase=body_retry_short duration_ms=%d err=%v", time.Since(t2).Milliseconds(), err)
			return strings.TrimSpace(newTitle), plainBody, err
		}
		newBody = strings.TrimSpace(newBody)
		ScrapeLogWithProviders("info", categorySlug, logURL, usedBody, "[scraper] AI API call ok phase=body_retry_short duration_ms=%d out_words=%d", time.Since(t2).Milliseconds(), countWordsLoose(newBody))
	}
	if isParaphraseTooShort(plainBody, newBody) {
		// Final guard: keep source body when AI repeatedly over-summarizes.
		// Keep AI output even if it's still shorter than expected;
		// falling back to the raw source makes it look like "no paraphrase".
	}

	if isParaphraseLikelyCopied(plainBody, newBody) {
		// If the model returns content that is effectively identical to the input,
		// retry with stronger anti-copy instructions.
		copyRetryPrompt := bodyPrompt + "\n\nInstruksi tambahan penting:\n- JANGAN menyalin kalimat asli secara verbatim.\n- Ubah struktur & urutan informasi.\n- Ganti frasa dengan sinonim sambil tetap mempertahankan fakta.\n- Output tidak boleh identik dengan input.\n"
		ScrapeLogWithProviders("info", categorySlug, logURL, "", "[scraper] AI API call start phase=body_retry_anticopy")
		t3 := time.Now()
		perCallCtx, cancel := context.WithTimeout(aiCtx, 75*time.Second)
		newBody2, usedAnti, err2 := scraperAIGenerate(perCallCtx, aiGen, p, copyRetryPrompt)
		cancel()
		if err2 != nil {
			ScrapeLogWithProviders("warn", categorySlug, logURL, usedAnti, "[scraper] AI API call failed phase=body_retry_anticopy duration_ms=%d err=%v", time.Since(t3).Milliseconds(), err2)
			return strings.TrimSpace(newTitle), plainBody, err2
		}
		newBody = strings.TrimSpace(newBody2)
		ScrapeLogWithProviders("info", categorySlug, logURL, usedAnti, "[scraper] AI API call ok phase=body_retry_anticopy duration_ms=%d out_words=%d", time.Since(t3).Milliseconds(), countWordsLoose(newBody))

		if isParaphraseLikelyCopied(plainBody, newBody) {
			return strings.TrimSpace(newTitle), plainBody, fmt.Errorf("AI output appears copied from source body")
		}
	}

	return strings.TrimSpace(newTitle), newBody, nil
}

func countWordsLoose(s string) int {
	s = strings.TrimSpace(spaceSeqRegex.ReplaceAllString(htmlTagRegex.ReplaceAllString(s, " "), " "))
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}

func isParaphraseTooShort(source, paraphrased string) bool {
	srcWords := countWordsLoose(source)
	if srcWords <= 0 {
		return false
	}
	paraWords := countWordsLoose(paraphrased)
	if paraWords <= 0 {
		return true
	}
	return float64(paraWords) < float64(srcWords)*0.85
}

func normalizeTextLooseForCopyCheck(s string) string {
	// Candidate is HTML; source is plain text.
	s = strings.ToLower(strings.TrimSpace(spaceSeqRegex.ReplaceAllString(htmlTagRegex.ReplaceAllString(s, " "), " ")))
	s = nonWordRegex.ReplaceAllString(s, " ")
	s = strings.TrimSpace(spaceSeqRegex.ReplaceAllString(s, " "))
	return s
}

func isParaphraseLikelyCopied(sourcePlain, candidateHTML string) bool {
	src := normalizeTextLooseForCopyCheck(sourcePlain)
	cand := normalizeTextLooseForCopyCheck(candidateHTML)
	if src == "" || cand == "" {
		return false
	}
	// Strong signal: AI output equals the original (after loose normalization).
	return src == cand
}

func clampRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:max]))
}

func normalizeAITitle(aiTitle string, fallback string) string {
	t := strings.TrimSpace(aiTitle)
	if t == "" {
		return strings.TrimSpace(fallback)
	}

	// Prefer a single-line title. Some LLMs respond with multi-paragraph "help" text.
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	t = strings.Trim(t, `"'“”`)
	t = strings.TrimSpace(t)
	if t == "" {
		return strings.TrimSpace(fallback)
	}

	const maxTitleRunes = 200 // must fit entity.Post.Title gorm size:200
	t = clampRunes(t, maxTitleRunes)
	if t == "" {
		return strings.TrimSpace(fallback)
	}
	return t
}

// normalizeTagDisplayName turns hyphens into spaces, collapses whitespace, and forces lowercase
// for scraper-generated tags.
func normalizeTagDisplayName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.TrimSpace(spaceSeqRegex.ReplaceAllString(s, " "))
	if s == "" {
		return ""
	}
	return strings.ToLower(s)
}

// firstTwoWordsFromPhrase returns up to the first two whitespace-separated words (trimmed, collapsed spaces).
// Hyphens count as word boundaries (e.g. "rio-fernandes berita" → "rio fernandes").
func firstTwoWordsFromPhrase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.TrimSpace(spaceSeqRegex.ReplaceAllString(s, " "))
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " " + parts[1]
}

func dedupeTagCandidatesFold(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	return out
}

// allMetaKeywordLabels builds ordered, deduped labels from comma-separated meta (first two words per segment).
// maxCandidates caps how many distinct labels we consider (DB match + fallback).
func allMetaKeywordLabels(meta string, maxCandidates int) []string {
	meta = strings.TrimSpace(meta)
	if meta == "" {
		return nil
	}
	candidates := make([]string, 0, 16)
	for _, part := range strings.Split(meta, ",") {
		w := firstTwoWordsFromPhrase(part)
		w = normalizeTagDisplayName(w)
		if w != "" {
			candidates = append(candidates, w)
		}
	}
	candidates = dedupeTagCandidatesFold(candidates)
	if maxCandidates > 0 && len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}
	return candidates
}

// scraperFallbackTagLabels returns up to maxTagsFromMetaKeywords labels for creating new tags when none exist in DB.
func scraperFallbackTagLabels(meta string) []string {
	all := allMetaKeywordLabels(meta, 24)
	if len(all) <= maxTagsFromMetaKeywords {
		return all
	}
	return all[:maxTagsFromMetaKeywords]
}

// scraperTitleTagStopwords filters junk tokens when deriving tags from titles (ID + EN glue words).
var scraperTitleTagStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "but": {}, "by": {},
	"for": {}, "from": {}, "has": {}, "have": {}, "in": {}, "is": {}, "it": {}, "its": {},
	"of": {}, "on": {}, "or": {}, "so": {}, "than": {}, "that": {}, "the": {}, "their": {},
	"this": {}, "to": {}, "was": {}, "were": {}, "what": {}, "when": {}, "which": {}, "who": {},
	"will": {}, "with": {},
	"adalah": {}, "akan": {}, "antara": {}, "atau": {}, "bagi": {}, "bahwa": {}, "bisa": {},
	"dalam": {}, "dan": {}, "dari": {}, "dengan": {}, "di": {}, "dia": {}, "dimana": {},
	"hanya": {}, "ini": {}, "itu": {}, "juga": {}, "jika": {}, "karena": {}, "ke": {}, "kepada": {},
	"lagi": {}, "lebih": {}, "mereka": {}, "oleh": {}, "pada": {}, "para": {}, "saja": {},
	"sang": {}, "satu": {}, "sebagai": {}, "sebuah": {}, "sebelum": {}, "seperti": {}, "setelah": {},
	"si": {}, "sudah": {}, "suatu": {}, "telah": {}, "tersebut": {}, "tidak": {}, "untuk": {},
	"yang": {},
}

func titleWordCandidatesForTags(title string) []string {
	title = strings.TrimSpace(htmlTagRegex.ReplaceAllString(title, " "))
	if title == "" {
		return nil
	}
	var b strings.Builder
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteByte(' ')
		}
	}
	parts := strings.Fields(b.String())
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if utf8.RuneCountInString(p) < 3 {
			continue
		}
		if _, stop := scraperTitleTagStopwords[p]; stop {
			continue
		}
		allDigit := true
		for _, r := range p {
			if !unicode.IsDigit(r) {
				allDigit = false
				break
			}
		}
		if allDigit {
			continue
		}
		out = append(out, p)
	}
	return dedupeTagCandidatesFold(out)
}

// scraperFallbackTagLabelsFromTitle picks up to max distinct words from the title (shuffled) when
// the source page has no usable meta keywords / tag matches.
func scraperFallbackTagLabelsFromTitle(title string, max int) []string {
	if max <= 0 {
		max = maxTagsFromMetaKeywords
	}
	cands := titleWordCandidatesForTags(title)
	if len(cands) == 0 {
		return nil
	}
	shuffled := append([]string(nil), cands...)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	if len(shuffled) > max {
		return shuffled[:max]
	}
	return shuffled
}

const scraperTagPlainExcerptMaxWords = 120

// takeFirstNWords returns a prefix of s containing at most n whitespace-separated words.
func takeFirstNWords(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" || n <= 0 {
		return ""
	}
	parts := strings.Fields(s)
	if len(parts) <= n {
		return s
	}
	return strings.Join(parts[:n], " ")
}

// scraperFallbackTagLabelsFromPlain derives tag labels from plain article text (e.g. paraphrased body)
// when meta keywords are missing or useless and title-based candidates are empty.
// Words are taken in document order (first meaningful tokens first).
func scraperFallbackTagLabelsFromPlain(plain string, max int) []string {
	if max <= 0 {
		max = maxTagsFromMetaKeywords
	}
	excerpt := takeFirstNWords(plain, scraperTagPlainExcerptMaxWords)
	cands := titleWordCandidatesForTags(excerpt)
	if len(cands) == 0 {
		return nil
	}
	if len(cands) > max {
		return cands[:max]
	}
	return cands
}

// findTagsOrderedByLabels returns tags already in the database whose name matches a label (case-insensitive, trim),
// in the order labels first appear. Duplicate tag IDs are skipped.
func findTagsOrderedByLabels(db *gorm.DB, labels []string) ([]entity.Tag, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	uniqLower := make([]string, 0, len(labels))
	seenIn := make(map[string]struct{}, len(labels))
	for _, lab := range labels {
		k := strings.ToLower(strings.TrimSpace(lab))
		if k == "" {
			continue
		}
		if _, ok := seenIn[k]; ok {
			continue
		}
		seenIn[k] = struct{}{}
		uniqLower = append(uniqLower, k)
	}
	if len(uniqLower) == 0 {
		return nil, nil
	}
	var rows []entity.Tag
	if err := db.Model(&entity.Tag{}).Where("LOWER(TRIM(name)) IN ?", uniqLower).Find(&rows).Error; err != nil {
		return nil, err
	}
	byLower := make(map[string]entity.Tag, len(rows))
	for _, t := range rows {
		byLower[strings.ToLower(strings.TrimSpace(t.Name))] = t
	}
	var out []entity.Tag
	seenID := make(map[uint]struct{})
	for _, lab := range labels {
		k := strings.ToLower(strings.TrimSpace(lab))
		t, ok := byLower[k]
		if !ok {
			continue
		}
		if _, dup := seenID[t.ID]; dup {
			continue
		}
		seenID[t.ID] = struct{}{}
		out = append(out, t)
	}
	return out, nil
}

// findAllTagsForAutoLinkScrape loads tag id/name/slug for in-body autolinking (longer names first).
func findAllTagsForAutoLinkScrape(db *gorm.DB, limit int) ([]entity.Tag, error) {
	if limit <= 0 {
		limit = 300
	}
	var out []entity.Tag
	err := db.Model(&entity.Tag{}).
		Select("id", "name", "slug").
		Where("CHAR_LENGTH(name) >= ?", 2).
		Order("CHAR_LENGTH(name) DESC, id ASC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

func mergeTagPoolForAutolink(attached []entity.Tag, pool []entity.Tag) []entity.Tag {
	byID := make(map[uint]entity.Tag)
	for _, t := range attached {
		if t.ID != 0 {
			byID[t.ID] = t
		}
	}
	for _, t := range pool {
		if t.ID == 0 {
			continue
		}
		if _, ok := byID[t.ID]; !ok {
			byID[t.ID] = t
		}
	}
	out := make([]entity.Tag, 0, len(byID))
	for _, t := range byID {
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].Name) > len(out[j].Name)
	})
	return out
}

func uniqueTagSlug(db *gorm.DB, base string) (string, error) {
	if base == "" {
		base = fmt.Sprintf("tag-%d", time.Now().UnixNano())
	}
	slug := base
	for i := 1; i <= 1000; i++ {
		var c int64
		if err := db.Model(&entity.Tag{}).Where("slug = ?", slug).Count(&c).Error; err != nil {
			return "", err
		}
		if c == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("failed to find unique tag slug for %q", base)
}

// ensureTagsByNames finds or creates tags by display name (order preserved). Names are clamped to tag column size.
func ensureTagsByNames(db *gorm.DB, names []string) ([]entity.Tag, error) {
	out := make([]entity.Tag, 0, len(names))
	for _, name := range names {
		name = normalizeTagDisplayName(strings.TrimSpace(name))
		name = firstTwoWordsFromPhrase(name)
		if name == "" {
			continue
		}
		name = clampRunes(name, 100)
		if name == "" {
			continue
		}

		var t entity.Tag
		// Use Find+Limit, not First: First logs "record not found" on every new tag.
		if err := db.Where("name = ?", name).Limit(1).Find(&t).Error; err != nil {
			return nil, err
		}
		if t.ID != 0 {
			out = append(out, t)
			continue
		}

		slugBase := slugify(name)
		slug, err := uniqueTagSlug(db, slugBase)
		if err != nil {
			return nil, err
		}
		t = entity.Tag{Name: name, Slug: slug}
		if err := db.Create(&t).Error; err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func createPostIfNew(
	ctx context.Context,
	client *http.Client,
	db *gorm.DB,
	aiGen *ai.Generator,
	store storage.ObjectStore,
	setting *runtimeSetting,
	in postCandidate,
	authorID uint,
	categoryID uint,
	includeSource bool,
	articleStatus string,
	imageMode string,
	webSearchSource string,
) (bool, error) {
	title := strings.TrimSpace(in.Title)
	link := strings.TrimSpace(in.Link)
	if title == "" || link == "" {
		ScrapeLogf("info", "", link, "[scraper] skip candidate: empty title/link title=%q link=%q", title, link)
		return false, nil
	}
	canonicalLink := canonicalizeScrapedArticleURL(link)
	storedSource := strings.TrimSpace(canonicalLink)
	if storedSource == "" {
		storedSource = strings.TrimSpace(link)
	}

	exists, err := postAlreadyExistsForScrapedSource(db, link, canonicalLink)
	if err != nil {
		return false, err
	}
	if exists {
		ScrapeLogf("info", "", link, "[scraper] skip duplicate by source link=%s", link)
		return false, nil
	}

	slug, err := uniqueSlug(db, slugify(title))
	if err != nil {
		return false, err
	}

	plain := strings.TrimSpace(spaceSeqRegex.ReplaceAllString(htmlTagRegex.ReplaceAllString(in.Description, " "), " "))
	if plain == "" {
		plain = ""
	}
	if w := countWordsLoose(plain); w < minScrapedContentWords {
		ScrapeLogf("info", "", link, "[scraper] skip candidate: scraped body too short (min=%d words) title=%q words=%d link=%s", minScrapedContentWords, title, w, link)
		return false, nil
	}
	aiEnabled := aiGen != nil && setting != nil && strings.TrimSpace(setting.ScraperAIProvider) != "" && strings.ToLower(strings.TrimSpace(setting.ScraperAIProvider)) != "off"
	if !aiEnabled {
		// AI is required for persistence: don't save non-paraphrased content.
		ScrapeLogf("info", "", link, "[scraper] skip candidate: AI disabled title=%q link=%s", title, link)
		return false, nil
	}
	t2, b2, err := paraphraseScrapedTitleAndBody(ctx, aiGen, setting, title, plain, in.CategorySlug, link, in.FeedURL)
	if err != nil {
		// If AI is enabled, don't persist the non-paraphrased body on failure.
		return false, err
	} else {
		title = normalizeAITitle(t2, title)
		if strings.TrimSpace(b2) != "" {
			plain = strings.TrimSpace(b2)
		}
		if title != "" {
			if slug2, err2 := uniqueSlug(db, slugify(title)); err2 == nil && slug2 != "" {
				slug = slug2
			}
		}
	}
	if w := countWordsLoose(plain); w < minScrapedContentWords {
		ScrapeLogf("info", "", link, "[scraper] skip candidate: final body too short after paraphrase (min=%d words) title=%q words=%d link=%s", minScrapedContentWords, title, w, link)
		return false, nil
	}
	content := plain
	imageToContent := false
	autoTag := repositories.ScraperAutoTagFromDB(db)
	if strings.TrimSpace(in.CategorySlug) != "" {
		var cf entity.ScraperCategoryFeed
		if err := db.Where("LOWER(TRIM(category_slug)) = ?", entity.CanonicalCategorySlug(in.CategorySlug)).First(&cf).Error; err == nil {
			imageToContent = cf.ImageToContent
		}
	}

	imgMode := strings.ToLower(strings.TrimSpace(imageMode))
	stat := strings.ToLower(strings.TrimSpace(articleStatus))
	if stat != "draft" {
		stat = "published"
	}
	storeMaxEdge, storeWebPQ := repositories.PostImageStoreLimitsFromDB(db)
	post := entity.Post{
		Title:         title,
		Slug:          slug,
		Content:       content,
		Source:        &storedSource,
		IncludeSource: includeSource,
		Status:        stat,
		AuthorID:      authorID,
		CategoryID:    categoryID,
	}

	var webRes *WebImageResult
	usedTemplateCover := false
	usedScrapeCover := false
	gotAICover := false
	switch imgMode {
	case "web_search":
		webCtx, webCancel := context.WithTimeout(ctx, 45*time.Second)
		webSrc := NormalizeWebSearchSource(webSearchSource)
		if webRes = BulkCoverImageResultFromWebSourcesAttemptSource(webCtx, db, store, title, slug, 0, webSrc); webRes != nil && strings.TrimSpace(webRes.StoredURL) != "" {
			u := strings.TrimSpace(webRes.StoredURL)
			post.Image = &u
			ScrapeLogf("info", "", link, "[scraper] web image source=%s title=%q link=%s image=%s", webSrc, title, link, u)
		}
		webCancel()
		if post.Image == nil || strings.TrimSpace(*post.Image) == "" {
			ScrapeLogf("warn", "", link, "[scraper] web image failed source=%s title=%q link=%s; using template cover (not source article image)", webSrc, title, link)
			if genURL, genErr := generateBulkCoverImageForScraper(ctx, db, store, title, slug); genErr == nil && strings.TrimSpace(genURL) != "" {
				post.Image = &genURL
				usedTemplateCover = true
			}
		}
	case "generate_image":
		imgCtx, imgCancel := context.WithTimeout(ctx, 2*time.Minute)
		aiURL, aiProvider, aiErr := generateAIPostCoverForScraper(imgCtx, db, store, setting, title, slug, false)
		imgCancel()
		if aiErr != nil {
			ScrapeLogWithProviders("warn", "", link, aiProvider, "[scraper] AI image failed %s title=%q link=%s err=%v", scraperImageAILogMessage("cover", aiProvider, ""), title, link, aiErr)
		} else if strings.TrimSpace(aiURL) == "" {
			ScrapeLogWithProviders("warn", "", link, aiProvider, "[scraper] AI image empty %s title=%q link=%s", scraperImageAILogMessage("cover", aiProvider, ""), title, link)
		} else if strings.TrimSpace(aiURL) != "" {
			post.Image = &aiURL
			gotAICover = true
			ScrapeLogWithProviders("info", "", link, aiProvider, "[scraper] AI image generated %s title=%q link=%s", scraperImageAILogMessage("cover", aiProvider, aiURL), title, link)
		}
		if post.Image == nil || strings.TrimSpace(*post.Image) == "" {
			if genURL, genErr := generateBulkCoverImageForScraper(ctx, db, store, title, slug); genErr == nil && strings.TrimSpace(genURL) != "" {
				post.Image = &genURL
				usedTemplateCover = true
			}
		}
		if post.Image == nil || strings.TrimSpace(*post.Image) == "" {
			post.Image = resolveScrapedPostImage(ctx, client, store, in, link, slug, storeMaxEdge, storeWebPQ)
			if post.Image != nil && strings.TrimSpace(*post.Image) != "" {
				usedScrapeCover = true
			}
		}
	case "generate":
		genURL, genErr := generateBulkCoverImageForScraper(ctx, db, store, title, slug)
		if genErr == nil && strings.TrimSpace(genURL) != "" {
			post.Image = &genURL
		} else {
			// Fallback to scraped image when template generation fails.
			post.Image = resolveScrapedPostImage(ctx, client, store, in, link, slug, storeMaxEdge, storeWebPQ)
			if post.Image != nil && strings.TrimSpace(*post.Image) != "" {
				usedScrapeCover = true
			}
		}
	default:
		// scrape/download image from article
		post.Image = resolveScrapedPostImage(ctx, client, store, in, link, slug, storeMaxEdge, storeWebPQ)
		if post.Image != nil && strings.TrimSpace(*post.Image) != "" {
			usedScrapeCover = true
		}
		if post.Image == nil || strings.TrimSpace(*post.Image) == "" {
			// Hosting environments may fail to fetch external images (WAF/CDN/network restrictions).
			// Fallback to generated cover so scrape still produces a post.
			if genURL, genErr := generateBulkCoverImageForScraper(ctx, db, store, title, slug); genErr == nil && strings.TrimSpace(genURL) != "" {
				post.Image = &genURL
				usedTemplateCover = true
			}
		}
	}

	assignScraperCoverImageOrigin(&post, imgMode, webRes, usedTemplateCover, usedScrapeCover, gotAICover, storedSource, strings.TrimSpace(in.HeroImageURL))

	if post.Image == nil || strings.TrimSpace(*post.Image) == "" {
		// Cancel scrape when image is missing or image download/generation failed.
		ScrapeLogf("info", "", link, "[scraper] skip candidate: no image title=%q link=%s", title, link)
		return false, nil
	}
	if imgMode == "generate_image" && imageToContent && post.Image != nil && strings.TrimSpace(*post.Image) != "" {
		// Match manual-post behavior: generate another image for in-content placement.
		contentImgURL := ""
		if setting != nil {
			contentSlug := slug + "-content"
			img2Ctx, img2Cancel := context.WithTimeout(ctx, 2*time.Minute)
			if u2, p2, err2 := generateAIPostCoverForScraper(img2Ctx, db, store, setting, title, contentSlug, true); err2 == nil && strings.TrimSpace(u2) != "" {
				contentImgURL = strings.TrimSpace(u2)
				ScrapeLogWithProviders("info", "", link, p2, "[scraper] AI image generated %s title=%q link=%s", scraperImageAILogMessage("content", p2, contentImgURL), title, link)
			} else if err2 != nil {
				ScrapeLogWithProviders("warn", "", link, p2, "[scraper] AI image failed %s title=%q link=%s err=%v", scraperImageAILogMessage("content", p2, ""), title, link, err2)
			} else {
				ScrapeLogWithProviders("warn", "", link, p2, "[scraper] AI image empty %s title=%q link=%s", scraperImageAILogMessage("content", p2, ""), title, link)
			}
			img2Cancel()
		}
		// Do not fall back to the cover image: if in-content generation fails, skip insertion
		// so cover and in-article images don't end up identical.
		if strings.TrimSpace(contentImgURL) != "" {
			content = insertImageIntoContentHTML(content, contentImgURL)
		}
	}
	categoryName := ""
	var cat entity.Category
	if err := db.First(&cat, categoryID).Error; err == nil {
		categoryName = strings.TrimSpace(cat.Name)
	}

	var tagsForPost []entity.Tag
	if autoTag {
		if strings.TrimSpace(in.MetaKeywords) != "" {
			labels := allMetaKeywordLabels(in.MetaKeywords, 24)
			var tagErr error
			tagsForPost, tagErr = findTagsOrderedByLabels(db, labels)
			if tagErr != nil {
				return false, tagErr
			}
			if len(tagsForPost) == 0 {
				fallback := scraperFallbackTagLabels(in.MetaKeywords)
				if len(fallback) > 0 {
					tagsForPost, tagErr = ensureTagsByNames(db, fallback)
					if tagErr != nil {
						return false, tagErr
					}
				}
			} else if len(tagsForPost) > maxTagsFromMetaKeywords {
				tagsForPost = tagsForPost[:maxTagsFromMetaKeywords]
			}
		}
		if len(tagsForPost) == 0 {
			titleFallback := scraperFallbackTagLabelsFromTitle(title, maxTagsFromMetaKeywords)
			if len(titleFallback) > 0 {
				var tagErr error
				tagsForPost, tagErr = ensureTagsByNames(db, titleFallback)
				if tagErr != nil {
					return false, tagErr
				}
			}
		}
		if len(tagsForPost) == 0 && strings.TrimSpace(content) != "" {
			bodyFallback := scraperFallbackTagLabelsFromPlain(content, maxTagsFromMetaKeywords)
			if len(bodyFallback) > 0 {
				var tagErr error
				tagsForPost, tagErr = ensureTagsByNames(db, bodyFallback)
				if tagErr != nil {
					return false, tagErr
				}
			}
		}
		if len(tagsForPost) == 0 && categoryName != "" {
			var tagErr error
			tagsForPost, tagErr = ensureTagsByNames(db, []string{categoryName})
			if tagErr != nil {
				return false, tagErr
			}
		}
	}
	if autoTag && len(tagsForPost) > 0 {
		pool, poolErr := findAllTagsForAutoLinkScrape(db, 300)
		if poolErr != nil {
			return false, poolErr
		}
		linkTags := mergeTagPoolForAutolink(tagsForPost, pool)
		content = tagcontent.AutoLinkHTML(content, linkTags, tagcontent.DefaultMaxTagLinks)
	}
	post.Content = content
	tagNamesForMeta := make([]string, 0, len(tagsForPost))
	for _, t := range tagsForPost {
		tagNamesForMeta = append(tagNamesForMeta, strings.TrimSpace(t.Name))
	}
	postmeta.Apply(&post, tagNamesForMeta, categoryName)
	if len(tagNamesForMeta) == 0 {
		if kw := strings.TrimSpace(in.MetaKeywords); kw != "" {
			kw = clampRunes(kw, 500)
			post.MetaKeywords = &kw
		}
	}

	// Re-check after AI/images/tags: another run may have inserted the same source in the meantime.
	existsLate, err := postAlreadyExistsForScrapedSource(db, link, canonicalLink)
	if err != nil {
		return false, err
	}
	if existsLate {
		ScrapeLogf("info", "", link, "[scraper] skip duplicate by source (recheck before save) link=%s", link)
		return false, nil
	}

	if err := alsoread.ApplyForPost(db, &post); err != nil {
		return false, err
	}
	if err := db.Create(&post).Error; err != nil {
		return false, err
	}
	if len(tagsForPost) > 0 {
		if err := db.Model(&post).Association("Tags").Append(tagsForPost); err != nil {
			return false, err
		}
	}
	return true, nil
}

func insertImageIntoContentHTML(content, imageURL string) string {
	content = strings.TrimSpace(content)
	imageURL = strings.TrimSpace(imageURL)
	if content == "" || imageURL == "" {
		return content
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"scraper-img-wrap\">" + content + "</div>"))
	if err != nil {
		return content
	}
	root := doc.Find("#scraper-img-wrap")
	if root.Length() == 0 {
		return content
	}
	if root.Find("img").FilterFunction(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		return strings.TrimSpace(src) == imageURL
	}).Length() > 0 {
		if out, e := root.Html(); e == nil {
			return strings.TrimSpace(out)
		}
		return content
	}
	imgHTML := `<p class="ai-inline-image-center" align="center"><img class="ai-inline-image" align="center" src="` + imageURL + `" alt="Generated image" loading="lazy"></p>`
	inserted := false
	h2s := root.Find("h2")
	if h2s.Length() >= 2 {
		h2s.Eq(1).AfterHtml(imgHTML)
		inserted = true
	}
	if !inserted {
		children := root.Children()
		if children.Length() == 0 {
			root.AppendHtml(imgHTML)
		} else {
			children.Eq(children.Length() / 2).BeforeHtml(imgHTML)
		}
	}
	if out, e := root.Html(); e == nil {
		return strings.TrimSpace(out)
	}
	return content
}

// collectSourceURLKeysForDedupe builds URL strings that might appear in post.source for the same article
// (raw feed link, canonical form, legacy www-prefixed host rows).
func collectSourceURLKeysForDedupe(link, canonical string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	addHTTPSSchemeFlip := func(s string) {
		add(s)
		u, err := url.Parse(s)
		if err != nil || u == nil || u.Host == "" {
			return
		}
		if strings.EqualFold(u.Scheme, "http") {
			u2 := *u
			u2.Scheme = "https"
			add(u2.String())
			return
		}
		if strings.EqualFold(u.Scheme, "https") {
			u2 := *u
			u2.Scheme = "http"
			add(u2.String())
		}
	}
	addHTTPSSchemeFlip(link)
	addHTTPSSchemeFlip(canonical)
	if canonical != "" {
		if u, err := url.Parse(canonical); err == nil && u.Host != "" {
			h := u.Host
			if !strings.HasPrefix(h, "www.") {
				u2 := *u
				u2.Host = "www." + h
				addHTTPSSchemeFlip(u2.String())
			}
		}
	}
	return out
}

func postAlreadyExistsForScrapedSource(db *gorm.DB, link, canonical string) (bool, error) {
	keys := collectSourceURLKeysForDedupe(link, canonical)
	if len(keys) == 0 {
		return false, nil
	}
	q := db.Model(&entity.Post{}).Where("source IN ?", keys)
	for _, k := range keys {
		if k == "" {
			continue
		}
		q = q.Or("source LIKE ?", k+"?%").
			Or("source = ?", k+"/").
			Or("source LIKE ?", k+"/?%")
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func canonicalizeScrapedArticleURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return strings.TrimSpace(raw)
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return strings.TrimSpace(raw)
	}
	u.Scheme = strings.ToLower(strings.TrimSpace(u.Scheme))
	host := strings.ToLower(strings.TrimSpace(u.Host))
	if strings.HasPrefix(host, "www.") {
		host = host[len("www."):]
	}
	u.Host = host
	u.Fragment = ""
	u.RawQuery = ""
	path := strings.TrimSpace(u.EscapedPath())
	if path == "/" {
		path = ""
	}
	path = strings.TrimSuffix(path, "/")
	u.Path = path
	u.RawPath = ""
	return u.String()
}

func cleanedContent(raw string, link string) string {
	plain := strings.TrimSpace(spaceSeqRegex.ReplaceAllString(htmlTagRegex.ReplaceAllString(raw, " "), " "))
	if plain == "" {
		return ""
	}
	return plain
}

func uniqueSlug(db *gorm.DB, base string) (string, error) {
	if base == "" {
		base = fmt.Sprintf("news-%d", time.Now().Unix())
	}
	slug := base
	for i := 1; i <= 1000; i++ {
		var c int64
		if err := db.Model(&entity.Post{}).Where("slug = ?", slug).Count(&c).Error; err != nil {
			return "", err
		}
		if c == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("failed to find unique slug for %q", base)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlugRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func atomEntryLink(entry atomEntry) string {
	for _, l := range entry.Links {
		if strings.EqualFold(l.Rel, "alternate") && strings.TrimSpace(l.Href) != "" {
			return l.Href
		}
	}
	for _, l := range entry.Links {
		if strings.TrimSpace(l.Href) != "" {
			return l.Href
		}
	}
	return ""
}

func resolveScraperAuthorByEmail(db *gorm.DB, email string) (uint, error) {
	email = strings.TrimSpace(email)
	if email != "" {
		var user entity.User
		if err := db.Where("email = ?", email).Limit(1).Find(&user).Error; err != nil {
			return 0, err
		}
		if user.ID != 0 {
			return user.ID, nil
		}
		ScrapeLogf("warn", "", "", "[scraper] author email %q not found; using fallback CMS user", email)
	}
	return resolveScraperFallbackAuthorID(db)
}

func resolveScraperFallbackAuthorID(db *gorm.DB) (uint, error) {
	for _, role := range []string{"admin", "writer"} {
		var user entity.User
		if err := db.Where("role = ? AND enabled = ?", role, true).Order("id asc").Limit(1).Find(&user).Error; err != nil {
			return 0, err
		}
		if user.ID != 0 {
			return user.ID, nil
		}
	}
	var user entity.User
	if err := db.Where("enabled = ?", true).Order("id asc").Limit(1).Find(&user).Error; err != nil {
		return 0, err
	}
	if user.ID == 0 {
		return 0, fmt.Errorf("no CMS user available as scraper author")
	}
	return user.ID, nil
}

func resolveCategoryID(db *gorm.DB, preferredSlug string) (uint, error) {
	preferredSlug = strings.TrimSpace(preferredSlug)
	if preferredSlug != "" {
		var category entity.Category
		if err := db.Where("slug = ?", preferredSlug).First(&category).Error; err == nil {
			return category.ID, nil
		}
	}

	var category entity.Category
	if err := db.Order("id asc").First(&category).Error; err != nil {
		return 0, fmt.Errorf("category not found: %w", err)
	}
	return category.ID, nil
}

func assignScraperCoverImageOrigin(post *entity.Post, imgMode string, webRes *WebImageResult, usedTemplateCover, usedScrapeCover, gotAICover bool, articleSource, heroURL string) {
	if post == nil || post.Image == nil || strings.TrimSpace(*post.Image) == "" {
		imagecaption.ClearPostImageOrigin(post)
		return
	}
	scrapeDetail := imagecaption.ScrapeSourceLabel(articleSource, heroURL)
	switch imgMode {
	case "web_search":
		if usedTemplateCover {
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginTemplate, "")
			return
		}
		detail := ""
		if webRes != nil {
			detail = webRes.Provider
		}
		imagecaption.SetPostImageOrigin(post, entity.ImageOriginWebSearch, detail)
	case "generate_image":
		switch {
		case gotAICover:
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginAI, "")
		case usedTemplateCover:
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginTemplate, "")
		case usedScrapeCover:
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginScrape, scrapeDetail)
		default:
			imagecaption.ClearPostImageOrigin(post)
		}
	case "generate":
		if usedScrapeCover {
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginScrape, scrapeDetail)
		} else {
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginTemplate, "")
		}
	default:
		if usedTemplateCover {
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginTemplate, "")
		} else if usedScrapeCover {
			imagecaption.SetPostImageOrigin(post, entity.ImageOriginScrape, scrapeDetail)
		} else {
			imagecaption.ClearPostImageOrigin(post)
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}
