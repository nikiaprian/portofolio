package alsoread

import (
	htmlpkg "html"
	"strings"

	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// DataAttr marks server-frozen "Baca juga" blocks (stripped and re-built on each publish).
const DataAttr = "data-tirtayasa-also-read"

const (
	paragraphInterval    = 3
	minRelatedFetchLimit = 36
	maxRelatedFetchLimit = 100
)

func relatedFetchLimitFor(maxInserts int) int {
	n := maxInserts * 6
	if n < minRelatedFetchLimit {
		return minRelatedFetchLimit
	}
	if n > maxRelatedFetchLimit {
		return maxRelatedFetchLimit
	}
	return n
}

func categoryPathForPost(articleSlugMode string, p *entity.Post) string {
	if p != nil && p.Category.ID != 0 {
		return entity.CategoryPathPrefixForArticleURL(articleSlugMode, p.Category)
	}
	return ""
}

// Strip removes all "Baca juga" aside blocks (server-frozen or legacy client-injected) before re-injecting.
func Strip(fragmentHTML string) string {
	fragmentHTML = strings.TrimSpace(fragmentHTML)
	if fragmentHTML == "" {
		return ""
	}
	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_tirtayasa_post_root">` + fragmentHTML + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return fragmentHTML
	}
	root := doc.Find("#_tirtayasa_post_root").First()
	if root.Length() == 0 {
		return fragmentHTML
	}
	root.Find("aside.blog-also-read").Remove()
	out, err := root.Html()
	if err != nil {
		return fragmentHTML
	}
	return strings.TrimSpace(out)
}

func buildAsideHTML(articleSlugMode string, target *entity.Post) string {
	if target == nil {
		return ""
	}
	href := entity.PublishedArticleWebPath(articleSlugMode, categoryPathForPost(articleSlugMode, target), target.Slug)
	title := strings.TrimSpace(target.Title)
	if href == "" || href == "/" || title == "" {
		return ""
	}
	return `<aside class="blog-also-read" aria-label="Baca juga" ` + DataAttr + `="1">` +
		`<div class="blog-also-read-inner">` +
		`<div class="blog-also-read-label">Baca juga</div>` +
		`<a class="blog-also-read-link" href="` + htmlpkg.EscapeString(href) + `">` +
		`<span class="blog-also-read-title">` + htmlpkg.EscapeString(title) + `</span>` +
		`</a></div></aside>`
}

func isTableOrListTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "table", "ol", "ul":
		return true
	default:
		return false
	}
}

// elementStartsWithTableOrList reports whether the first non-empty child element
// of n is (or recursively wraps) table/ol/ul — used to detect wrappers so "Baca
// juga" can be placed after the whole block instead of before it.
func elementStartsWithTableOrList(n *nethtml.Node) bool {
	if n == nil {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case nethtml.TextNode:
			if strings.TrimSpace(c.Data) == "" {
				continue
			}
			return false
		case nethtml.CommentNode:
			continue
		case nethtml.ElementNode:
			tag := c.Data
			if isTableOrListTag(tag) {
				return true
			}
			if tag == "div" || tag == "figure" {
				if elementStartsWithTableOrList(c) {
					return true
				}
			}
			return false
		default:
			continue
		}
	}
	return false
}

// followingTableOrListBlockAfterP returns the first element sibling of p that is
// a table/list or a div/figure whose first block content is table/list.
func followingTableOrListBlockAfterP(p *nethtml.Node) *nethtml.Node {
	if p == nil || p.Type != nethtml.ElementNode || !strings.EqualFold(p.Data, "p") {
		return nil
	}
	for sib := p.NextSibling; sib != nil; sib = sib.NextSibling {
		switch sib.Type {
		case nethtml.TextNode:
			if strings.TrimSpace(sib.Data) == "" {
				continue
			}
			return nil
		case nethtml.CommentNode:
			continue
		case nethtml.ElementNode:
			tag := sib.Data
			if isTableOrListTag(tag) {
				return sib
			}
			if (tag == "div" || tag == "figure") && elementStartsWithTableOrList(sib) {
				return sib
			}
			return nil
		default:
			continue
		}
	}
	return nil
}

// paragraphFollowedByTableOrList is true when the next block sibling of p is a
// table, list, or a div/figure that starts with one of those.
func paragraphFollowedByTableOrList(p *nethtml.Node) bool {
	return followingTableOrListBlockAfterP(p) != nil
}

// nextStructuredBlockSibling returns the first following sibling of cur that is
// table/ol/ul or a div/figure whose first block child is table/ol/ul.
func nextStructuredBlockSibling(cur *goquery.Selection) *goquery.Selection {
	if cur == nil || cur.Length() == 0 {
		return nil
	}
	for {
		nxt := cur.Next()
		if nxt.Length() == 0 {
			return nil
		}
		node := nxt.Get(0)
		if node == nil {
			return nil
		}
		switch node.Type {
		case nethtml.TextNode:
			if strings.TrimSpace(node.Data) == "" {
				cur = nxt
				continue
			}
			return nil
		case nethtml.CommentNode:
			cur = nxt
			continue
		case nethtml.ElementNode:
			tag := node.Data
			if isTableOrListTag(tag) {
				return nxt
			}
			if (tag == "div" || tag == "figure") && elementStartsWithTableOrList(node) {
				return nxt
			}
			return nil
		default:
			cur = nxt
		}
	}
}

// nextInsertionAnchorAfterP returns the node after which to insert "Baca juga"
// when the paragraph is immediately followed by a table or list (insert after
// that block); otherwise nil so callers use the paragraph as anchor.
func nextInsertionAnchorAfterP(pEl *goquery.Selection) *goquery.Selection {
	if pEl.Get(0) == nil || !paragraphFollowedByTableOrList(pEl.Get(0)) {
		return nil
	}
	return nextStructuredBlockSibling(pEl)
}

// relocateAlsoReadAfterFollowingBlocks moves aside.blog-also-read that sit
// immediately before a table or list to after that block (legacy / edge HTML).
func relocateAlsoReadAfterFollowingBlocks(root *goquery.Selection) {
	if root == nil || root.Length() == 0 {
		return
	}
	var asides []*goquery.Selection
	root.Find("aside.blog-also-read").Each(func(_ int, s *goquery.Selection) {
		if s.Length() > 0 {
			asides = append(asides, s)
		}
	})
	for _, as := range asides {
		n := as.Get(0)
		if n == nil || !strings.EqualFold(n.Data, "aside") {
			continue
		}
		blockSel := nextStructuredBlockSibling(as)
		if blockSel == nil || blockSel.Length() == 0 {
			continue
		}
		htmlFrag, err := goquery.OuterHtml(as)
		if err != nil || strings.TrimSpace(htmlFrag) == "" {
			continue
		}
		as.Remove()
		blockSel.AfterHtml(htmlFrag)
	}
}

// InjectFrozen inserts up to maxInserts frozen "Baca juga" blocks after every Nth non-empty <p>.
func InjectFrozen(fragmentHTML, articleSlugMode string, related []entity.Post, maxInserts int) string {
	maxInserts = entity.EffectiveAlsoReadMaxInserts(maxInserts)
	fragmentHTML = strings.TrimSpace(fragmentHTML)
	if fragmentHTML == "" || len(related) == 0 {
		return fragmentHTML
	}
	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_tirtayasa_post_root">` + fragmentHTML + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return fragmentHTML
	}
	root := doc.Find("#_tirtayasa_post_root").First()
	if root.Length() == 0 {
		return fragmentHTML
	}

	paras := root.Find("p")
	if paras.Length() == 0 {
		out, err := root.Html()
		if err != nil {
			return fragmentHTML
		}
		return strings.TrimSpace(out)
	}

	insertIdx := 0
	inserted := 0
	seen := make(map[uint]struct{})
	count := 0

	paras.Each(func(_ int, pEl *goquery.Selection) {
		if inserted >= maxInserts {
			return
		}
		txt := strings.TrimSpace(pEl.Text())
		if txt == "" {
			return
		}
		count++
		if count%paragraphInterval != 0 {
			return
		}
		for insertIdx < len(related) {
			rp := &related[insertIdx]
			if rp != nil && rp.ID != 0 {
				if _, ok := seen[rp.ID]; !ok {
					break
				}
			}
			insertIdx++
		}
		if insertIdx >= len(related) {
			return
		}
		post := &related[insertIdx]
		insertIdx++
		if post == nil || post.ID == 0 {
			return
		}
		seen[post.ID] = struct{}{}
		block := buildAsideHTML(articleSlugMode, post)
		if block == "" {
			return
		}
		anchor := pEl
		if a := nextInsertionAnchorAfterP(pEl); a != nil && a.Length() > 0 {
			anchor = a
		}
		anchor.AfterHtml(block)
		inserted++
	})

	relocateAlsoReadAfterFollowingBlocks(root)

	out, err := root.Html()
	if err != nil {
		return fragmentHTML
	}
	return strings.TrimSpace(out)
}
