package postutil

import (
	"bytes"
	"html"
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	htmlpkg "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// DefaultMaxTagLinks limits how many <a href="/tag/..."> are inserted per HTML pass (first matches in document order).
const DefaultMaxTagLinks = 3

type tagLinkCandidate struct {
	name string
	slug string
}

type tagMatch struct {
	start int
	end   int
	cand  tagLinkCandidate
}

// AutoLinkHTML wraps tag names (whole words) in <a href="/tag/{slug}"> inside HTML text nodes.
// Skips the first <p> in document order (no autolinks in the opening paragraph). Skips text inside existing <a> tags.
// When maxLinks > 0, at most that many links are added; maxLinks <= 0 means no limit.
func AutoLinkHTML(content string, tags []entity.Tag, maxLinks int) string {
	content = strings.TrimSpace(content)
	if content == "" || len(tags) == 0 {
		return content
	}

	candidates := make([]tagLinkCandidate, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		name := strings.TrimSpace(t.Name)
		slug := strings.TrimSpace(t.Slug)
		if name == "" || slug == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, tagLinkCandidate{name: name, slug: slug})
	}
	if len(candidates) == 0 {
		return content
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return len(candidates[i].name) > len(candidates[j].name)
	})

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

	var remaining *int
	if maxLinks > 0 {
		r := maxLinks
		remaining = &r
	}
	usedTag := make(map[string]struct{}, len(candidates))
	firstP := findFirstPElement(root)
	if countPElements(root) <= 1 {
		firstP = nil
	}
	linkTagsInTextNodes(root, false, false, firstP, candidates, remaining, usedTag)

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

func linkTagsInTextNodes(n *htmlpkg.Node, insideAnchor, skipAutolink bool, firstPElement *htmlpkg.Node, candidates []tagLinkCandidate, remaining *int, usedTag map[string]struct{}) {
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
			replaceTextNodeWithTagLinks(c, candidates, remaining, usedTag)
		} else {
			linkTagsInTextNodes(c, isAnchor, skipAutolink, firstPElement, candidates, remaining, usedTag)
		}
		c = next
	}
}

func replaceTextNodeWithTagLinks(n *htmlpkg.Node, candidates []tagLinkCandidate, remaining *int, usedTag map[string]struct{}) {
	if n == nil || n.Parent == nil {
		return
	}
	text := n.Data
	if strings.TrimSpace(text) == "" {
		return
	}

	type segment struct {
		text string
		tag  *tagLinkCandidate
	}
	segments := make([]segment, 0, 8)
	cursor := 0
	for cursor < len(text) {
		if remaining != nil && *remaining <= 0 {
			if cursor < len(text) {
				segments = append(segments, segment{text: text[cursor:]})
			}
			break
		}
		m, ok := findNextTagMatch(text, cursor, candidates, usedTag)
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
		segments = append(segments, segment{text: matchText, tag: &m.cand})
		cursor = m.end
		if remaining != nil {
			*remaining--
		}
		usedTag[strings.ToLower(strings.TrimSpace(m.cand.slug))] = struct{}{}
	}

	if len(segments) == 1 && segments[0].tag == nil {
		return
	}

	parent := n.Parent
	for _, seg := range segments {
		if seg.text == "" {
			continue
		}
		if seg.tag == nil {
			parent.InsertBefore(&htmlpkg.Node{Type: htmlpkg.TextNode, Data: seg.text}, n)
			continue
		}
		href := "/tag/" + url.PathEscape(strings.TrimSpace(seg.tag.slug))
		a := &htmlpkg.Node{
			Type: htmlpkg.ElementNode,
			Data: "a",
			Attr: []htmlpkg.Attribute{{Key: "href", Val: href}},
		}
		a.AppendChild(&htmlpkg.Node{Type: htmlpkg.TextNode, Data: seg.text})
		parent.InsertBefore(a, n)
	}
	parent.RemoveChild(n)
}

func findNextTagMatch(text string, start int, candidates []tagLinkCandidate, usedTag map[string]struct{}) (tagMatch, bool) {
	if start >= len(text) {
		return tagMatch{}, false
	}
	lower := strings.ToLower(text)
	best := tagMatch{}
	found := false
	for _, cand := range candidates {
		slugKey := strings.ToLower(strings.TrimSpace(cand.slug))
		if slugKey != "" {
			if _, already := usedTag[slugKey]; already {
				continue
			}
		}
		name := strings.TrimSpace(cand.name)
		if name == "" {
			continue
		}
		target := strings.ToLower(name)
		searchFrom := start
		for searchFrom < len(lower) {
			rel := strings.Index(lower[searchFrom:], target)
			if rel < 0 {
				break
			}
			i := searchFrom + rel
			j := i + len(target)
			if tagMatchHasWordBoundaries(text, i, j) {
				cur := tagMatch{start: i, end: j, cand: cand}
				if !found || cur.start < best.start || (cur.start == best.start && len(cand.name) > len(best.cand.name)) {
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

func tagMatchHasWordBoundaries(s string, start int, end int) bool {
	if start < 0 || end > len(s) || start >= end {
		return false
	}
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:start])
		if isTagWordRune(r) {
			return false
		}
	}
	if end < len(s) {
		r, _ := utf8.DecodeRuneInString(s[end:])
		if isTagWordRune(r) {
			return false
		}
	}
	return true
}

func isTagWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
