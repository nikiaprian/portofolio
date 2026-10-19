package posthtml

import (
	"strings"

	"golang.org/x/net/html"
)

const blogTableScrollClass = "blog-table-scroll"

// WrapTablesForMobileScroll wraps bare <table> nodes in a horizontal scroll container
// so wide tables are not clipped on small screens.
func WrapTablesForMobileScroll(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.Contains(strings.ToLower(raw), "<table") {
		return raw
	}
	root, err := parseFragmentRoot(raw)
	if err != nil || root == nil {
		return raw
	}
	wrapTablesInNode(root)
	return renderInnerHTML(root)
}

func wrapTablesInNode(n *html.Node) {
	if n == nil {
		return
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode {
			if strings.EqualFold(c.Data, "table") && !isInsideTableScroll(c) {
				wrap := &html.Node{
					Type: html.ElementNode,
					Data: "div",
					Attr: []html.Attribute{{Key: "class", Val: blogTableScrollClass}},
				}
				parent := c.Parent
				if parent != nil {
					parent.InsertBefore(wrap, c)
					detachNode(c)
					wrap.AppendChild(c)
				}
			} else {
				wrapTablesInNode(c)
			}
		}
		c = next
	}
}

func isInsideTableScroll(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && strings.EqualFold(p.Data, "div") {
			cls := strings.ToLower(attrVal(p, "class"))
			if strings.Contains(cls, blogTableScrollClass) {
				return true
			}
		}
	}
	return false
}
