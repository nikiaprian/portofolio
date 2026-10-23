package autohref

import (
	"bytes"
	"html"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	htmlpkg "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"gorm.io/gorm"
)

type hrefCandidate struct {
	text string
	url  string
}

type matchResult struct {
	start int
	end   int
	cand  hrefCandidate
}

// ApplyFromDB loads site_settings and runs Apply when auto_href_enabled is true.
func ApplyFromDB(db *gorm.DB, currentPostID uint, content string) string {
	if db == nil {
		return content
	}
	ss := repositories.SiteSettingOrDefault(db)
	if !ss.AutoHrefEnabled {
		return content
	}
	return Apply(db, ss, currentPostID, content)
}

// Apply parses content (HTML), fetches candidates from the DB, and replaces matches with hyperlinks.
// Category names link to /{category-slug}. Article titles link via PublishedArticleWebPath:
//   - article_slug_mode=direct: /{post-slug}
//   - article_slug_mode=category: /{category-slug}/{post-slug}
//   - article_slug_mode=category_nested: /{parent-slug}/{sub-slug}/{post-slug}
// Legacy /post/{slug} is not used (that path only 301-redirects to the canonical URL).
func Apply(db *gorm.DB, siteSetting entity.SiteSetting, currentPostID uint, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}

	// 1. Fetch categories
	var categories []entity.Category
	if err := db.Order("name asc").Find(&categories).Error; err != nil {
		return content
	}

	// 2. Fetch other published posts (limit to 500 latest to prevent memory issues)
	var posts []entity.Post
	if err := db.Preload("Category").
		Where("id != ? AND status = ?", currentPostID, "published").
		Order("created_at desc").
		Limit(500).
		Find(&posts).Error; err != nil {
		return content
	}

	// 3. Build Category Candidates
	catCandidates := make([]hrefCandidate, 0, len(categories))
	for _, cat := range categories {
		name := strings.TrimSpace(cat.Name)
		slug := strings.TrimSpace(cat.Slug)
		if name == "" || slug == "" {
			continue
		}
		catCandidates = append(catCandidates, hrefCandidate{
			text: name,
			url:  entity.PublishedCategoryWebPath(slug),
		})
	}

	// 4. Build Article Candidates
	artCandidates := make([]hrefCandidate, 0, len(posts))
	for _, p := range posts {
		title := strings.TrimSpace(p.Title)
		if len(title) < 3 { // skip very short titles
			continue
		}
		catPrefix := ""
		if p.Category.ID != 0 {
			catPrefix = entity.CategoryPathPrefixForArticleURL(siteSetting.ArticleSlugMode, p.Category)
		}
		href := entity.PublishedArticleWebPath(siteSetting.ArticleSlugMode, catPrefix, p.Slug)
		artCandidates = append(artCandidates, hrefCandidate{
			text: title,
			url:  href,
		})
	}

	return ApplyWithCandidates(content, catCandidates, artCandidates)
}

// ApplyWithCandidates runs the auto-linking on content using the provided candidates.
func ApplyWithCandidates(content string, catCandidates, artCandidates []hrefCandidate) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}

	// Sort categories by name length descending so longer ones match first
	sort.SliceStable(catCandidates, func(i, j int) bool {
		return len(catCandidates[i].text) > len(catCandidates[j].text)
	})

	// Sort articles by title length descending so longer ones match first
	sort.SliceStable(artCandidates, func(i, j int) bool {
		return len(artCandidates[i].text) > len(artCandidates[j].text)
	})

	// Parse HTML fragment
	parseSrc := content
	if !strings.Contains(content, "<") {
		parseSrc = "<p>" + html.EscapeString(content) + "</p>"
	}

	ctx := &htmlpkg.Node{
		Type:     htmlpkg.ElementNode,
		DataAtom: atom.Div,
		Data:     atom.Div.String(),
	}
	nodes, err := htmlpkg.ParseFragment(strings.NewReader(parseSrc), ctx)
	if err != nil {
		return content
	}
	root := &htmlpkg.Node{
		Type:     htmlpkg.ElementNode,
		DataAtom: atom.Div,
		Data:     atom.Div.String(),
	}
	for _, n := range nodes {
		root.AppendChild(n)
	}

	firstP := findFirstPElement(root)
	// Skip the lead paragraph only when there is more than one <p> (single-block articles should still be linkable).
	if countPElements(root) <= 1 {
		firstP = nil
	}

	// Pass 1: Auto-link Articles (Max 3 links) - Run articles first so longer title matches are not broken by shorter category terms
	remainingArts := 3
	linkCandidatesInTextNodes(root, false, false, firstP, artCandidates, &remainingArts)

	// Pass 2: Auto-link Categories (Max 3 links)
	remainingCats := 3
	linkCandidatesInTextNodes(root, false, false, firstP, catCandidates, &remainingCats)

	// Render back to HTML
	var out strings.Builder
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		var b bytes.Buffer
		if err := htmlpkg.Render(&b, c); err != nil {
			return content
		}
		out.Write(b.Bytes())
	}
	return strings.TrimSpace(out.String())
}

func countPElements(root *htmlpkg.Node) int {
	if root == nil {
		return 0
	}
	n := 0
	var walk func(*htmlpkg.Node)
	walk = func(node *htmlpkg.Node) {
		if node == nil {
			return
		}
		if node.Type == htmlpkg.ElementNode && strings.EqualFold(node.Data, "p") {
			n++
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return n
}

func findFirstPElement(root *htmlpkg.Node) *htmlpkg.Node {
	if root == nil {
		return nil
	}
	var walk func(*htmlpkg.Node) *htmlpkg.Node
	walk = func(n *htmlpkg.Node) *htmlpkg.Node {
		if n == nil {
			return nil
		}
		if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "p") {
			return n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if hit := walk(c); hit != nil {
				return hit
			}
		}
		return nil
	}
	return walk(root)
}

func linkCandidatesInTextNodes(n *htmlpkg.Node, insideAnchor, skipAutolink bool, firstPElement *htmlpkg.Node, candidates []hrefCandidate, remaining *int) {
	if n == nil {
		return
	}
	if !skipAutolink && firstPElement != nil && n == firstPElement {
		skipAutolink = true
	}
	isAnchor := insideAnchor || (n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "a"))

	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == htmlpkg.TextNode && !isAnchor && !skipAutolink {
			replaceTextNodeWithHrefLinks(c, candidates, remaining)
		} else {
			linkCandidatesInTextNodes(c, isAnchor, skipAutolink, firstPElement, candidates, remaining)
		}
		c = next
	}
}

func replaceTextNodeWithHrefLinks(n *htmlpkg.Node, candidates []hrefCandidate, remaining *int) {
	if n == nil || n.Parent == nil {
		return
	}
	text := n.Data
	if strings.TrimSpace(text) == "" {
		return
	}

	type segment struct {
		text string
		cand *hrefCandidate
	}
	var segments []segment
	cursor := 0
	for cursor < len(text) {
		if remaining != nil && *remaining <= 0 {
			if cursor < len(text) {
				segments = append(segments, segment{text: text[cursor:]})
			}
			break
		}

		var m matchResult
		var ok bool
		// Articles use exact title match (same as categories) so href always matches the real canonical URL.
		m, ok = findNextExactMatch(text, cursor, candidates)

		if !ok {
			if cursor < len(text) {
				segments = append(segments, segment{text: text[cursor:]})
			}
			break
		}

		if m.start > cursor {
			segments = append(segments, segment{text: text[cursor:m.start]})
		}
		matchText := text[m.start:m.end]
		segments = append(segments, segment{text: matchText, cand: &m.cand})
		cursor = m.end
		if remaining != nil {
			*remaining--
		}
	}

	if len(segments) == 1 && segments[0].cand == nil {
		return
	}

	parent := n.Parent
	for _, seg := range segments {
		if seg.text == "" {
			continue
		}
		if seg.cand == nil {
			parent.InsertBefore(&htmlpkg.Node{Type: htmlpkg.TextNode, Data: seg.text}, n)
			continue
		}
		a := &htmlpkg.Node{
			Type: htmlpkg.ElementNode,
			Data: "a",
			Attr: []htmlpkg.Attribute{{Key: "href", Val: seg.cand.url}},
		}
		a.AppendChild(&htmlpkg.Node{Type: htmlpkg.TextNode, Data: seg.text})
		parent.InsertBefore(a, n)
	}
	parent.RemoveChild(n)
}

func findNextExactMatch(text string, start int, candidates []hrefCandidate) (matchResult, bool) {
	if start >= len(text) {
		return matchResult{}, false
	}
	lower := strings.ToLower(text)
	best := matchResult{}
	found := false
	for _, cand := range candidates {
		target := strings.ToLower(cand.text)
		searchFrom := start
		for searchFrom < len(lower) {
			rel := strings.Index(lower[searchFrom:], target)
			if rel < 0 {
				break
			}
			i := searchFrom + rel
			j := i + len(target)
			if hasWordBoundaries(text, i, j) {
				cur := matchResult{start: i, end: j, cand: cand}
				if !found || cur.start < best.start || (cur.start == best.start && len(cand.text) > len(best.cand.text)) {
					best = cur
					found = true
				}
				break
			}
			searchFrom = i + len(target)
		}
	}
	return best, found
}

func hasWordBoundaries(s string, start int, end int) bool {
	if start < 0 || end > len(s) || start >= end {
		return false
	}
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:start])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	if end < len(s) {
		r, _ := utf8.DecodeRuneInString(s[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
