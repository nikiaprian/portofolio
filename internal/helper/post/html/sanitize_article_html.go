package posthtml

import (
	"strings"

	"golang.org/x/net/html"
)

// SanitizePublicArticleHTML re-parses article HTML into a well-formed fragment and strips
// ChatGPT/UI chrome wrappers that leak </div></section> into the page and break footer layout.
func SanitizePublicArticleHTML(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	root, err := parseFragmentRoot(raw)
	if err != nil || root == nil {
		return raw
	}
	sanitizeArticleNode(root)
	return renderInnerHTML(root)
}

func sanitizeArticleNode(n *html.Node) {
	if n == nil {
		return
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.CommentNode {
			detachNode(c)
			c = next
			continue
		}
		if c.Type == html.ElementNode {
			if isRemovableChromeNode(c) {
				detachNode(c)
				c = next
				continue
			}
			sanitizeArticleNode(c)
			if isUnwrapChromeNode(c) {
				unwrapNode(c)
			} else {
				cleanArticleAttrs(c)
			}
		}
		c = next
	}
}

func unwrapNode(n *html.Node) {
	if n == nil || n.Parent == nil {
		return
	}
	parent := n.Parent
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		detachNode(c)
		parent.InsertBefore(c, n)
		c = next
	}
	detachNode(n)
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func classList(n *html.Node) string {
	return strings.ToLower(attrVal(n, "class"))
}

func isBlankText(s string) bool {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.TrimSpace(s) == ""
}

func isRemovableChromeNode(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	cls := classList(n)
	tag := strings.ToLower(n.Data)

	if tag == "style" || tag == "link" {
		return true
	}
	if tag == "img" {
		src := strings.TrimSpace(strings.ToLower(attrVal(n, "src")))
		if strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "blob:") {
			return true
		}
	}

	// Empty ChatGPT spacers / scroll sentinels only — never drop nodes that still hold article text.
	if strings.Contains(cls, "empty:hidden") && !hasMeaningfulContent(n) {
		return true
	}
	if isChatGPTScrollSentinel(cls) && !hasMeaningfulContent(n) {
		return true
	}
	if tag == "div" && strings.Contains(cls, "qmyqug_convsearchresulthighlightroot") && !hasMeaningfulContent(n) {
		return true
	}
	return false
}

func isChatGPTScrollSentinel(cls string) bool {
	return strings.Contains(cls, "pointer-events-none") &&
		(strings.Contains(cls, "h-px") || strings.Contains(cls, "-mt-px") || strings.Contains(cls, "translate-y-"))
}

func isUnwrapChromeNode(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	cls := classList(n)
	tag := strings.ToLower(n.Data)

	if tag != "div" && tag != "section" && tag != "article" {
		return false
	}

	chromeMarks := []string{
		"tyaggw_tablecontainer",
		"tyaggw_tablewrapper",
		"qmyqug_",
		"pdq2pg_",
		"r6vx5w_",
		"threadscrollvars",
		"text-token-text-primary",
		"agent-turn",
		"[--thread-content",
		"thread-content-margin",
		"thread-content-max-width",
		"group/turn-messages",
		"text-message",
		"markdown-new-styling",
	}
	for _, m := range chromeMarks {
		if strings.Contains(cls, m) {
			return true
		}
	}
	// ChatGPT sometimes nests real headings inside scroll-sentinel wrappers.
	if isChatGPTScrollSentinel(cls) {
		return true
	}
	if attrVal(n, "data-conversation-screenshot-content") != "" {
		return true
	}
	if attrVal(n, "data-turn") != "" || attrVal(n, "data-turn-id") != "" || attrVal(n, "data-turn-id-container") != "" {
		return true
	}
	if attrVal(n, "data-message-author-role") != "" || attrVal(n, "data-message-id") != "" {
		return true
	}
	if strings.HasPrefix(attrVal(n, "data-testid"), "conversation-turn") {
		return true
	}
	// Generic markdown shell from ChatGPT copy.
	if strings.Contains(cls, "markdown") && strings.Contains(cls, "prose") {
		return true
	}
	return false
}

func hasMeaningfulContent(n *html.Node) bool {
	var walk func(*html.Node) bool
	walk = func(x *html.Node) bool {
		if x == nil {
			return false
		}
		if x.Type == html.TextNode && !isBlankText(x.Data) {
			return true
		}
		if x.Type == html.ElementNode {
			switch strings.ToLower(x.Data) {
			case "p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "table", "thead", "tbody", "tr", "td", "th", "img", "figure", "blockquote", "pre", "code", "a", "strong", "em":
				// Element itself counts if it can carry content; check descendants too.
				if strings.EqualFold(x.Data, "img") {
					return true
				}
			}
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			if walk(c) {
				return true
			}
		}
		return false
	}
	return walk(n)
}

func cleanArticleAttrs(n *html.Node) {
	if n == nil || n.Type != html.ElementNode {
		return
	}
	dropKeys := map[string]bool{
		"data-start":                           true,
		"data-end":                             true,
		"data-section-id":                      true,
		"data-col-size":                        true,
		"data-is-last-node":                    true,
		"data-is-only-node":                    true,
		"data-testid":                          true,
		"data-turn":                            true,
		"data-turn-id":                         true,
		"data-turn-id-container":               true,
		"data-conversation-screenshot-content": true,
	}
	out := n.Attr[:0]
	for _, a := range n.Attr {
		k := strings.ToLower(a.Key)
		if dropKeys[k] || strings.HasPrefix(k, "data-turn") {
			continue
		}
		if k == "style" && strings.Contains(strings.ToLower(a.Val), "mso-") {
			continue
		}
		if k == "class" {
			cleaned := cleanChromeClasses(a.Val)
			cleaned = stripEditorJunkClasses(cleaned)
			if cleaned == "" {
				continue
			}
			a.Val = cleaned
		}
		out = append(out, a)
	}
	n.Attr = out
}

func cleanChromeClasses(class string) string {
	parts := strings.Fields(class)
	if len(parts) == 0 {
		return ""
	}
	keep := make([]string, 0, len(parts))
	for _, p := range parts {
		lp := strings.ToLower(p)
		if strings.Contains(lp, "tyaggw_") ||
			strings.Contains(lp, "pdq2pg_") ||
			strings.Contains(lp, "qmyqug_") ||
			strings.Contains(lp, "r6vx5w_") ||
			strings.Contains(lp, "threadscrollvars") ||
			strings.Contains(lp, "text-token-text-primary") ||
			strings.Contains(lp, "agent-turn") ||
			strings.Contains(lp, "thread-content") ||
			strings.Contains(lp, "selectionanchor") ||
			strings.HasPrefix(lp, "[--thread") ||
			strings.Contains(lp, "empty:hidden") ||
			strings.Contains(lp, "group/turn") {
			continue
		}
		keep = append(keep, p)
	}
	return strings.Join(keep, " ")
}

// stripEditorJunkClasses removes paste artifacts from editors/AI tools (not from bebii templates).
func stripEditorJunkClasses(class string) string {
	parts := strings.Fields(class)
	if len(parts) == 0 {
		return ""
	}
	junk := map[string]bool{
		"isselectedend":   true,
		"isselectedstart": true,
		"isselected":      true,
	}
	keep := make([]string, 0, len(parts))
	for _, p := range parts {
		lp := strings.ToLower(p)
		if junk[lp] ||
			strings.HasPrefix(lp, "mso") ||
			strings.HasPrefix(lp, "docs-") ||
			strings.HasPrefix(lp, "kix_") ||
			strings.HasPrefix(lp, "lst-kix_") ||
			strings.HasPrefix(lp, "apple-") {
			continue
		}
		keep = append(keep, p)
	}
	return strings.Join(keep, " ")
}
