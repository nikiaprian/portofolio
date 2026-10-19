package autohref

import (
	"net/url"
	"strings"

	htmlpkg "golang.org/x/net/html"
)

func articleURLKey(href string) string {
	return strings.ToLower(strings.TrimSpace(href))
}

// anchorHrefPathKey normalizes href for comparison (relative path or URL path only).
func anchorHrefPathKey(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		if u, err := url.Parse(href); err == nil && strings.TrimSpace(u.Path) != "" {
			return articleURLKey(u.Path)
		}
	}
	return articleURLKey(href)
}

func anchorHrefFromNode(n *htmlpkg.Node) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "href") {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func categorySlugKey(href string) string {
	p := strings.TrimSpace(href)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if strings.HasPrefix(strings.ToLower(p), "/tag/") {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(p, "/"), "/")
}

// collectTagAutolinkKeys returns lowercased tag slugs and anchor texts already linked via /tag/... (scraper autotag).
func collectTagAutolinkKeys(root *htmlpkg.Node) map[string]struct{} {
	keys := make(map[string]struct{})
	if root == nil {
		return keys
	}
	var walk func(*htmlpkg.Node)
	walk = func(n *htmlpkg.Node) {
		if n == nil {
			return
		}
		if n.Type == htmlpkg.ElementNode && strings.EqualFold(n.Data, "a") {
			href := ""
			for _, a := range n.Attr {
				if strings.EqualFold(a.Key, "href") {
					href = strings.TrimSpace(a.Val)
					break
				}
			}
			const tagPrefix = "/tag/"
			if len(href) >= len(tagPrefix) && strings.EqualFold(href[:len(tagPrefix)], tagPrefix) {
				slug := href[len(tagPrefix):]
				if i := strings.IndexByte(slug, '/'); i >= 0 {
					slug = slug[:i]
				}
				if i := strings.IndexByte(slug, '?'); i >= 0 {
					slug = slug[:i]
				}
				if unescaped, err := url.PathUnescape(slug); err == nil {
					slug = unescaped
				}
				slug = strings.ToLower(strings.TrimSpace(slug))
				if slug != "" {
					keys[slug] = struct{}{}
				}
				var text strings.Builder
				var textWalk func(*htmlpkg.Node)
				textWalk = func(tn *htmlpkg.Node) {
					if tn == nil {
						return
					}
					if tn.Type == htmlpkg.TextNode {
						text.WriteString(tn.Data)
					}
					for c := tn.FirstChild; c != nil; c = c.NextSibling {
						textWalk(c)
					}
				}
				textWalk(n)
				if name := strings.ToLower(strings.TrimSpace(text.String())); name != "" {
					keys[name] = struct{}{}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return keys
}

func hrefCandidateURLSet(candidates []hrefCandidate) map[string]struct{} {
	set := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		if key := articleURLKey(c.url); key != "" {
			set[key] = struct{}{}
		}
	}
	return set
}

func isStripableAutoHrefAnchor(n *htmlpkg.Node, urls map[string]struct{}) bool {
	if n == nil || n.Type != htmlpkg.ElementNode || !strings.EqualFold(n.Data, "a") || len(urls) == 0 {
		return false
	}
	href := anchorHrefFromNode(n)
	if href == "" {
		return false
	}
	_, ok := urls[anchorHrefPathKey(href)]
	return ok
}

func unwrapAnchor(n *htmlpkg.Node) {
	if n == nil || n.Parent == nil {
		return
	}
	parent := n.Parent
	var text strings.Builder
	var extract func(*htmlpkg.Node)
	extract = func(node *htmlpkg.Node) {
		if node == nil {
			return
		}
		if node.Type == htmlpkg.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	parent.InsertBefore(&htmlpkg.Node{Type: htmlpkg.TextNode, Data: text.String()}, n)
	parent.RemoveChild(n)
}

// stripAutoHrefAnchors removes prior autohref <a href="..."> (href-only) so Apply can re-link from plain text.
func stripAutoHrefAnchors(root *htmlpkg.Node, urls map[string]struct{}) {
	if root == nil || len(urls) == 0 {
		return
	}
	var strip func(*htmlpkg.Node)
	strip = func(n *htmlpkg.Node) {
		if n == nil {
			return
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if isStripableAutoHrefAnchor(c, urls) {
				unwrapAnchor(c)
			} else {
				strip(c)
			}
			c = next
		}
	}
	strip(root)
}

func filterCategoriesNotTagLinked(candidates []hrefCandidate, tagLinked map[string]struct{}) []hrefCandidate {
	if len(tagLinked) == 0 {
		return candidates
	}
	out := make([]hrefCandidate, 0, len(candidates))
	for _, c := range candidates {
		nameKey := strings.ToLower(strings.TrimSpace(c.text))
		if nameKey != "" {
			if _, skip := tagLinked[nameKey]; skip {
				continue
			}
		}
		if slug := strings.ToLower(categorySlugKey(c.url)); slug != "" {
			if _, skip := tagLinked[slug]; skip {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}
