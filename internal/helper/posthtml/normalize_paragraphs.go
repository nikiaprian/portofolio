// Package posthtml normalizes AI/editor HTML body structure (paragraph breaks).
package posthtml

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var reWhitespaceBetweenTags = regexp.MustCompile(`>[ \t\r\n\f]+<`)

var reBrTag = regexp.MustCompile(`(?is)<br\s*/?>`)

// Split rich <p> inner HTML on <br> or line breaks (including nested inside strong/a).
var rePhrasingParagraphBreaks = regexp.MustCompile(`(?is)(?:<br\s*/?>\s*)+|(?:\r\n|\n|\r)`)

func stripBrTagsToSpace(s string) string {
	root, err := parseFragmentRoot(s)
	if err != nil || root == nil {
		return s
	}
	scan := scanRootForNormalize(root)
	changed := replaceParagraphBrTagsWithSpace(scan)
	if !changed {
		return s
	}
	out := renderInnerHTML(scan)
	if strings.TrimSpace(out) == "" {
		return s
	}
	return out
}

func replaceParagraphBrTagsWithSpace(p *html.Node) bool {
	changed := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == html.ElementNode && strings.EqualFold(c.Data, "br") {
				parent := c.Parent
				mark := c.NextSibling
				detachNode(c)
				insertBefore(parent, &html.Node{Type: html.TextNode, Data: " "}, mark)
				changed = true
				c = next
				continue
			}
			walk(c)
			c = next
		}
	}
	walk(p)
	return changed
}

// NormalizeParagraphRows mirrors the admin post form "Normalize rows" pipeline:
// 1) every <p>: turn <br> / newlines in text into separate <p> (no <br> left)
// 2) strip raw newlines in the serialized HTML
// 3) split inline-only <p> into 2-sentence blocks while preserving inline tags
func NormalizeParagraphRows(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return in
	}
	s1 := wrapTopLevelPhrasingRuns(in)
	s2 := convertBrAndNewlinesToParagraphs(s1)
	s2 = stripBrTagsToSpace(s2)
	s3 := stripRawLineBreaks(s2)
	s4 := reflowLongPlainParagraphs(s3)
	return strings.TrimSpace(stripRawLineBreaks(s4))
}

func stripNewlinesBetweenTags(s string) string {
	if s == "" {
		return s
	}
	return reWhitespaceBetweenTags.ReplaceAllString(s, "><")
}

func stripRawLineBreaks(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return stripNewlinesBetweenTags(s)
}

func wrapTopLevelPhrasingRuns(htmlStr string) string {
	root, err := parseFragmentRoot(htmlStr)
	if err != nil || root == nil {
		return htmlStr
	}
	scan := scanRootForNormalize(root)
	if !wrapPhrasingRunsInParagraphs(scan) {
		return htmlStr
	}
	out := renderInnerHTML(scan)
	if strings.TrimSpace(out) == "" {
		return htmlStr
	}
	return out
}

func wrapPhrasingRunsInParagraphs(parent *html.Node) bool {
	if parent == nil {
		return false
	}
	changed := false
	var run []*html.Node
	flush := func(before *html.Node) {
		run = trimPhrasingRun(run)
		if len(run) == 0 {
			return
		}
		p := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P}
		for _, n := range run {
			appendChild(p, n)
		}
		insertBefore(parent, p, before)
		run = nil
		changed = true
	}

	for c := parent.FirstChild; c != nil; {
		next := c.NextSibling
		if isTopLevelPhrasingNode(c) {
			run = append(run, c)
		} else {
			flush(c)
		}
		c = next
	}
	flush(nil)
	return changed
}

func trimPhrasingRun(run []*html.Node) []*html.Node {
	for len(run) > 0 && isBlankTextNode(run[0]) {
		detachNode(run[0])
		run = run[1:]
	}
	for len(run) > 0 && isBlankTextNode(run[len(run)-1]) {
		detachNode(run[len(run)-1])
		run = run[:len(run)-1]
	}
	return run
}

func isTopLevelPhrasingNode(n *html.Node) bool {
	if n == nil {
		return false
	}
	switch n.Type {
	case html.TextNode:
		return strings.TrimSpace(n.Data) != ""
	case html.CommentNode:
		return true
	case html.ElementNode:
		return inlinePhrasingTag(n.Data)
	default:
		return false
	}
}

func isBlankTextNode(n *html.Node) bool {
	return n != nil && n.Type == html.TextNode && strings.TrimSpace(n.Data) == ""
}

func convertBrAndNewlinesToParagraphs(htmlStr string) string {
	root, err := parseFragmentRoot(htmlStr)
	if err != nil || root == nil {
		return htmlStr
	}
	scan := scanRootForNormalize(root)
	ps := collectParagraphsPreOrder(scan)
	changed := false
	for i := len(ps) - 1; i >= 0; i-- {
		p := ps[i]
		if p.Parent == nil {
			continue
		}
		repl := expandParagraphBreaks(p)
		if len(repl) == 1 && repl[0] == p {
			continue
		}
		if len(repl) == 0 {
			continue
		}
		parent := p.Parent
		mark := p.NextSibling
		detachNode(p)
		for _, n := range repl {
			insertBefore(parent, n, mark)
		}
		changed = true
	}
	if !changed {
		return htmlStr
	}
	out := renderInnerHTML(scan)
	if strings.TrimSpace(out) == "" {
		return htmlStr
	}
	return out
}

func collectParagraphsPreOrder(root *html.Node) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "p") {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func insertBefore(parent, node, before *html.Node) {
	if parent == nil || node == nil {
		return
	}
	detachNode(node)
	node.Parent = parent
	if before == nil {
		appendChild(parent, node)
		return
	}
	node.NextSibling = before
	node.PrevSibling = before.PrevSibling
	if before.PrevSibling != nil {
		before.PrevSibling.NextSibling = node
	} else {
		parent.FirstChild = node
	}
	before.PrevSibling = node
}

func expandParagraphBreaks(ch *html.Node) []*html.Node {
	if ch.Type != html.ElementNode || !strings.EqualFold(ch.Data, "p") {
		return []*html.Node{ch}
	}
	if pContainsBr(ch) {
		if isPlainPhrasingParagraph(ch) {
			return expandPlainParagraphBreaks(ch)
		}
		return splitMixedParagraphAtBRs(ch)
	}
	if plainPHasLineBreaks(ch) || subtreeHasNewlineText(ch) {
		if isPlainPhrasingParagraph(ch) {
			return expandPlainParagraphBreaks(ch)
		}
		return splitMixedParagraphAtBRs(ch)
	}
	return []*html.Node{ch}
}

func subtreeHasNewlineText(n *html.Node) bool {
	if n.Type == html.TextNode && strings.ContainsAny(n.Data, "\r\n") {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if subtreeHasNewlineText(c) {
			return true
		}
	}
	return false
}

func pContainsBr(n *html.Node) bool {
	var walk func(*html.Node) bool
	walk = func(x *html.Node) bool {
		if x.Type == html.ElementNode && strings.EqualFold(x.Data, "br") {
			return true
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

// splitMixedParagraphAtBRs splits <p> on <br> / newlines anywhere under the paragraph
// (e.g. inside <strong>, <a>) by parsing serialized inner HTML fragments.
func splitMixedParagraphAtBRs(p *html.Node) []*html.Node {
	inner := renderChildrenHTML(p)
	if strings.TrimSpace(inner) == "" {
		return []*html.Node{p}
	}
	segs := rePhrasingParagraphBreaks.Split(inner, -1)
	if len(segs) <= 1 {
		return []*html.Node{p}
	}
	var out []*html.Node
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		np := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P}
		nodes, err := html.ParseFragment(strings.NewReader(seg), np)
		if err != nil {
			continue
		}
		for _, n := range nodes {
			appendChild(np, n)
		}
		if np.FirstChild == nil && strings.TrimSpace(seg) != "" {
			appendChild(np, &html.Node{Type: html.TextNode, Data: seg})
		}
		if np.FirstChild == nil {
			continue
		}
		out = append(out, np)
	}
	if len(out) == 0 {
		return []*html.Node{p}
	}
	if len(out) == 1 {
		detachChildren(p)
		for c := out[0].FirstChild; c != nil; {
			next := c.NextSibling
			appendChild(p, c)
			c = next
		}
		return []*html.Node{p}
	}
	return out
}

func renderChildrenHTML(p *html.Node) string {
	var buf bytes.Buffer
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return ""
		}
	}
	return buf.String()
}

func detachChildren(parent *html.Node) {
	for c := parent.FirstChild; c != nil; {
		next := c.NextSibling
		detachNode(c)
		c = next
	}
}

func reflowLongPlainParagraphs(htmlStr string) string {
	root, err := parseFragmentRoot(htmlStr)
	if err != nil || root == nil {
		return htmlStr
	}
	scan := scanRootForNormalize(root)
	ps := collectParagraphsPreOrder(scan)
	changed := false
	const group = 2
	for i := len(ps) - 1; i >= 0; i-- {
		p := ps[i]
		if p.Parent == nil {
			continue
		}
		repl := reflowInlineChild(p, group)
		if len(repl) == 1 && repl[0] == p {
			continue
		}
		if len(repl) == 0 {
			continue
		}
		parent := p.Parent
		mark := p.NextSibling
		detachNode(p)
		for _, n := range repl {
			insertBefore(parent, n, mark)
		}
		changed = true
	}
	if !changed {
		return htmlStr
	}
	out := renderInnerHTML(scan)
	if strings.TrimSpace(out) == "" {
		return htmlStr
	}
	return out
}

func reflowInlineChild(ch *html.Node, group int) []*html.Node {
	if ch.Type != html.ElementNode || !strings.EqualFold(ch.Data, "p") || !isInlinePhrasingSubtreeOnly(ch) {
		return []*html.Node{ch}
	}
	if group <= 0 {
		group = 2
	}
	textMap := buildNormalizedTextMap(ch)
	sents := splitNormalizedTextIntoSentenceRanges(textMap.text)
	if len(sents) <= group {
		return []*html.Node{ch}
	}
	var out []*html.Node
	for g := 0; g < len(sents); g += group {
		end := g + group
		if end > len(sents) {
			end = len(sents)
		}
		rawStart := textMap.before[sents[g].start]
		rawEnd := textMap.after[sents[end-1].end-1]
		np := cloneParagraphTextRange(ch, rawStart, rawEnd)
		if np.FirstChild == nil || collapsePlainText(textContent(np)) == "" {
			continue
		}
		out = append(out, np)
	}
	if len(out) == 0 {
		return []*html.Node{ch}
	}
	return out
}

type textRange struct {
	start int
	end   int
}

type normalizedTextMap struct {
	text   string
	before []int
	after  []int
}

func buildNormalizedTextMap(n *html.Node) normalizedTextMap {
	var chars []rune
	var before []int
	var after []int
	rawOffset := 0
	prevSpace := true
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			for i, r := range x.Data {
				size := utf8.RuneLen(r)
				if size < 0 {
					size = 1
				}
				rawBefore := rawOffset + i
				rawAfter := rawBefore + size
				if unicode.IsSpace(r) || r == '\u00a0' {
					if !prevSpace {
						chars = append(chars, ' ')
						before = append(before, rawBefore)
						after = append(after, rawAfter)
						prevSpace = true
					}
					continue
				}
				chars = append(chars, r)
				before = append(before, rawBefore)
				after = append(after, rawAfter)
				prevSpace = false
			}
			rawOffset += len(x.Data)
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	if len(chars) > 0 && chars[len(chars)-1] == ' ' {
		chars = chars[:len(chars)-1]
		before = before[:len(before)-1]
		after = after[:len(after)-1]
	}
	return normalizedTextMap{text: string(chars), before: before, after: after}
}

func splitNormalizedTextIntoSentenceRanges(t string) []textRange {
	t = strings.TrimSpace(t)
	if t == "" {
		return nil
	}
	runes := []rune(t)
	n := len(runes)
	var ranges []textRange
	start := 0
	for j := 0; j < n; j++ {
		if runes[j] != '.' {
			continue
		}
		if isDecimalPeriod(runes, j) || isDomainLikePeriod(runes, j) {
			continue
		}
		sp := j + 1
		for sp < n && runes[sp] == ' ' {
			sp++
		}
		if sp < n {
			ranges = append(ranges, textRange{start: start, end: j + 1})
			start = sp
		}
	}
	if start < n {
		ranges = append(ranges, textRange{start: start, end: n})
	}
	return ranges
}

func isDecimalPeriod(runes []rune, i int) bool {
	return i > 0 && unicode.IsDigit(runes[i-1]) && i+1 < len(runes) && unicode.IsDigit(runes[i+1])
}

func isDomainLikePeriod(runes []rune, i int) bool {
	if i <= 0 || i+1 >= len(runes) {
		return false
	}
	if !isDomainLabelRune(runes[i-1]) || !isDomainLabelRune(runes[i+1]) {
		return false
	}

	left := i - 1
	for left >= 0 && isDomainLabelRune(runes[left]) {
		left--
	}
	right := i + 1
	for right < len(runes) && isDomainLabelRune(runes[right]) {
		right++
	}
	return hasLetter(runes[left+1:i]) && hasLetter(runes[i+1:right])
}

func isDomainLabelRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-'
}

func hasLetter(runes []rune) bool {
	for _, r := range runes {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func cloneParagraphTextRange(p *html.Node, start, end int) *html.Node {
	np := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P}
	pos := 0
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if cloned := cloneNodeTextRange(c, start, end, &pos); cloned != nil {
			appendChild(np, cloned)
		}
	}
	return np
}

func cloneNodeTextRange(n *html.Node, start, end int, pos *int) *html.Node {
	if n.Type == html.TextNode {
		nodeStart := *pos
		nodeEnd := nodeStart + len(n.Data)
		*pos = nodeEnd
		if end <= nodeStart || start >= nodeEnd {
			return nil
		}
		from := maxInt(start, nodeStart) - nodeStart
		to := minInt(end, nodeEnd) - nodeStart
		if from >= to {
			return nil
		}
		return &html.Node{Type: html.TextNode, Data: n.Data[from:to]}
	}
	if n.Type != html.ElementNode {
		return nil
	}
	clone := &html.Node{Type: html.ElementNode, Data: n.Data, DataAtom: n.DataAtom, Attr: append([]html.Attribute(nil), n.Attr...)}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if child := cloneNodeTextRange(c, start, end, pos); child != nil {
			appendChild(clone, child)
		}
	}
	if clone.FirstChild == nil {
		return nil
	}
	return clone
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func expandPlainParagraphBreaks(ch *html.Node) []*html.Node {
	if ch.Type != html.ElementNode || !strings.EqualFold(ch.Data, "p") || !isPlainPhrasingParagraph(ch) {
		return []*html.Node{ch}
	}
	segs := splitPlainPOnBreaks(ch)
	had := plainPHasLineBreaks(ch)
	if len(segs) == 0 {
		return []*html.Node{ch}
	}
	if len(segs) == 1 && !had {
		return []*html.Node{ch}
	}
	var out []*html.Node
	for _, t := range segs {
		out = append(out, newParagraph(t))
	}
	return out
}

func newParagraph(text string) *html.Node {
	p := &html.Node{Type: html.ElementNode, Data: "p", DataAtom: atom.P}
	tn := &html.Node{Type: html.TextNode, Data: text}
	appendChild(p, tn)
	return p
}

func isPlainPhrasingParagraph(n *html.Node) bool {
	if n.Type != html.ElementNode || !strings.EqualFold(n.Data, "p") {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.TextNode:
			continue
		case html.ElementNode:
			if strings.EqualFold(c.Data, "br") {
				continue
			}
			return false
		default:
			return false
		}
	}
	return true
}

func isInlinePhrasingSubtreeOnly(n *html.Node) bool {
	if n.Type != html.ElementNode || !strings.EqualFold(n.Data, "p") {
		return false
	}
	var walk func(*html.Node) bool
	walk = func(x *html.Node) bool {
		switch x.Type {
		case html.TextNode:
			return true
		case html.CommentNode:
			return true
		case html.ElementNode:
			if !inlinePhrasingTag(x.Data) {
				return false
			}
			for c := x.FirstChild; c != nil; c = c.NextSibling {
				if !walk(c) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !walk(c) {
			return false
		}
	}
	return true
}

func inlinePhrasingTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "a", "abbr", "b", "bdi", "bdo", "basefont", "br", "cite", "code", "data", "del", "dfn", "em", "font", "i", "ins", "kbd", "mark", "nobr", "q", "ruby", "rp", "rt", "s", "samp", "small", "span", "strike", "strong", "sub", "sup", "time", "tt", "u", "var", "wbr", "big":
		return true
	default:
		return false
	}
}

func plainPHasLineBreaks(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "br") {
			return true
		}
		if c.Type == html.TextNode && strings.ContainsAny(c.Data, "\r\n") {
			return true
		}
	}
	return false
}

func splitPlainPOnBreaks(p *html.Node) []string {
	var segs []string
	var cur strings.Builder
	flush := func() {
		t := collapsePlainText(cur.String())
		cur.Reset()
		if t != "" {
			segs = append(segs, t)
		}
	}
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			parts := strings.FieldsFunc(c.Data, func(r rune) bool {
				return r == '\r' || r == '\n'
			})
			for i, part := range parts {
				if i > 0 {
					flush()
				}
				cur.WriteString(part)
			}
		} else if c.Type == html.ElementNode && strings.EqualFold(c.Data, "br") {
			flush()
		}
	}
	flush()
	return segs
}

func collapsePlainText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), " ")
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func splitPlainTextIntoSentences(t string) []string {
	t = collapsePlainText(t)
	if t == "" {
		return nil
	}
	runes := []rune(t)
	n := len(runes)
	capStart := func(r rune) bool {
		if r >= 'A' && r <= 'Z' {
			return true
		}
		return unicode.IsLetter(r) && unicode.IsUpper(r)
	}
	var sents []string
	start := 0
	for j := 0; j < n; j++ {
		if runes[j] != '.' {
			continue
		}
		if j > 0 && unicode.IsDigit(runes[j-1]) && j+1 < n && unicode.IsDigit(runes[j+1]) {
			continue
		}
		sp := j + 1
		for sp < n && runes[sp] == ' ' {
			sp++
		}
		if sp < n && capStart(runes[sp]) {
			piece := strings.TrimSpace(string(runes[start : j+1]))
			if piece != "" {
				sents = append(sents, piece)
			}
			start = sp
		}
	}
	last := strings.TrimSpace(string(runes[start:]))
	if last != "" {
		sents = append(sents, last)
	}
	if len(sents) == 0 {
		return []string{t}
	}
	return sents
}

func parseFragmentRoot(htmlStr string) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader("<!DOCTYPE html><html><head></head><body>" + htmlStr + "</body></html>"))
	if err != nil {
		return nil, err
	}
	var body *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "body") {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	if body == nil {
		return nil, errors.New("posthtml: no body")
	}
	wrap := &html.Node{Type: html.ElementNode, Data: "div"}
	for c := body.FirstChild; c != nil; {
		next := c.NextSibling
		appendChild(wrap, c)
		c = next
	}
	return wrap, nil
}

func elementChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

func scanRootForNormalize(root *html.Node) *html.Node {
	ech := elementChildren(root)
	if len(ech) == 1 && strings.EqualFold(ech[0].Data, "div") {
		return ech[0]
	}
	return root
}

func detachNode(n *html.Node) {
	if n == nil {
		return
	}
	p := n.Parent
	if p == nil {
		return
	}
	if p.FirstChild == n {
		p.FirstChild = n.NextSibling
	}
	if n.PrevSibling != nil {
		n.PrevSibling.NextSibling = n.NextSibling
	}
	if n.NextSibling != nil {
		n.NextSibling.PrevSibling = n.PrevSibling
	}
	n.Parent = nil
	n.PrevSibling = nil
	n.NextSibling = nil
}

func appendChild(parent, child *html.Node) {
	detachNode(child)
	child.Parent = parent
	child.PrevSibling = nil
	child.NextSibling = nil
	if parent.FirstChild == nil {
		parent.FirstChild = child
		return
	}
	last := parent.FirstChild
	for last.NextSibling != nil {
		last = last.NextSibling
	}
	last.NextSibling = child
	child.PrevSibling = last
}

func renderInnerHTML(parent *html.Node) string {
	var buf bytes.Buffer
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return ""
		}
	}
	// goquery/TinyMCE-friendly: escape text the same way as browser innerHTML for simple paragraphs.
	s := buf.String()
	return strings.TrimSpace(s)
}
