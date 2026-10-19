package posthtml

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	aiFenceRe        = regexp.MustCompile("(?s)^\\s*```(?:html|HTML|markdown|md)?\\s*\\n(.*)\\n```\\s*$")
	aiFenceLooseRe   = regexp.MustCompile("(?s)```(?:html|HTML)?\\s*\\n(.*?)\\n```")
	aiHTMLBlockTagRe = regexp.MustCompile(`(?i)</?(?:p|h[1-6]|ul|ol|li|table|blockquote|div|section)\b`)
	aiMDHeadingRe    = regexp.MustCompile(`(?m)^#{1,6}\s+\S`)
	aiMDBoldRe       = regexp.MustCompile(`\*\*[^*]+\*\*|__[^_]+__`)
	aiPlanPhraseRe   = regexp.MustCompile(`(?i)plan mode|rencana artikel|berikut rencana|menyusun rencana|tidak akan menulis|keluar dari plan|kalau rencana ini|hanya menyusun|sebelum menulis`)
)

// SanitizeAIDraftHTML cleans model output intended as a CMS article body:
// strip code fences / plan-mode chatter, convert markdown to HTML when needed,
// then re-parse through SanitizePublicArticleHTML.
func SanitizeAIDraftHTML(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = stripAIMarkdownFences(s)
	s = stripAIPlanPreamble(s)
	if looksLikeMarkdownArticle(s) {
		s = markdownToHTMLFragment(s)
	}
	s = SanitizePublicArticleHTML(s)
	return strings.TrimSpace(s)
}

// LooksLikeAIPlanOnly reports whether the model returned an outline/plan instead of article HTML.
func LooksLikeAIPlanOnly(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return true
	}
	if !aiPlanPhraseRe.MatchString(s) {
		return false
	}
	cleaned := SanitizeAIDraftHTML(s)
	if cleaned == "" {
		return true
	}
	if !aiHTMLBlockTagRe.MatchString(cleaned) {
		return true
	}
	// Plan leftovers that barely became HTML (short outline).
	return utf8.RuneCountInString(cleaned) < 400
}

func stripAIMarkdownFences(s string) string {
	if m := aiFenceRe.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	// Multiple fences: keep the largest fenced block when the outer text is mostly chatter.
	if aiFenceLooseRe.MatchString(s) && !aiHTMLBlockTagRe.MatchString(s) {
		best := ""
		for _, m := range aiFenceLooseRe.FindAllStringSubmatch(s, -1) {
			if len(m) == 2 && utf8.RuneCountInString(m[1]) > utf8.RuneCountInString(best) {
				best = m[1]
			}
		}
		if strings.TrimSpace(best) != "" {
			return strings.TrimSpace(best)
		}
	}
	return strings.TrimSpace(s)
}

func stripAIPlanPreamble(s string) string {
	if !aiPlanPhraseRe.MatchString(s) {
		return s
	}
	// Prefer first real HTML body tag.
	if loc := aiHTMLBlockTagRe.FindStringIndex(s); loc != nil && loc[0] > 0 {
		head := strings.ToLower(s[:loc[0]])
		if aiPlanPhraseRe.MatchString(head) {
			return strings.TrimSpace(s[loc[0]:])
		}
	}
	// Prefer first markdown heading that looks like article section (not "Rencana...").
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "#") {
			continue
		}
		low := strings.ToLower(t)
		if strings.Contains(low, "rencana") || strings.Contains(low, "plan") || strings.Contains(low, "catatan") || strings.Contains(low, "verifikasi") {
			continue
		}
		if i > 0 {
			return strings.TrimSpace(strings.Join(lines[i:], "\n"))
		}
	}
	return s
}

func looksLikeMarkdownArticle(s string) bool {
	if aiHTMLBlockTagRe.MatchString(s) {
		// Already HTML-dominant: only convert when markdown headings still dominate.
		htmlHits := len(aiHTMLBlockTagRe.FindAllStringIndex(s, -1))
		mdHits := len(aiMDHeadingRe.FindAllStringIndex(s, -1))
		if htmlHits >= mdHits && htmlHits >= 2 {
			return false
		}
	}
	return aiMDHeadingRe.MatchString(s) || (aiMDBoldRe.MatchString(s) && strings.Contains(s, "\n"))
}

func markdownToHTMLFragment(s string) string {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, extension.Strikethrough),
		goldmark.WithRendererOptions(html.WithHardWraps()),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		return s
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return s
	}
	return out
}
