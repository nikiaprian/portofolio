package posthtml

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ArticleAdSlot describes one in-article ad with its target position (1..99 = percent of article paragraphs).
type ArticleAdSlot struct {
	Position int    `json:"position"`
	Content  string `json:"content"`
}

// ParseArticleAdsJSON decodes the stored JSON representation of article ads. Returns nil
// when the input is empty or malformed; the caller treats nil as "no ads configured".
func ParseArticleAdsJSON(raw string) []ArticleAdSlot {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	var rows []ArticleAdSlot
	if err := json.Unmarshal([]byte(s), &rows); err != nil {
		return nil
	}
	out := make([]ArticleAdSlot, 0, len(rows))
	for _, r := range rows {
		if r.Position < 1 || r.Position > 99 {
			continue
		}
		if strings.TrimSpace(r.Content) == "" {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out
}

const articleAdMarkerPrefix = "TIRTAYASA_ARTICLE_AD_MARKER_"

var articleAdMarkerRe = regexp.MustCompile(`<!--\s*` + articleAdMarkerPrefix + `(\d+)\s*-->`)

// InjectArticleAds rewrites the post body HTML by inserting each configured ad slot after the
// paragraph that ends at its percentage position. The ad content (which may contain <script>)
// is round-tripped via HTML comment markers so that goquery's serializer cannot mangle it.
//
// When the body has no top-level <p> elements, all ads are appended to the end of the body so they
// are still visible.
func InjectArticleAds(bodyHTML string, slots []ArticleAdSlot) string {
	if len(slots) == 0 || strings.TrimSpace(bodyHTML) == "" {
		return bodyHTML
	}
	ordered := make([]ArticleAdSlot, 0, len(slots))
	for _, s := range slots {
		if strings.TrimSpace(s.Content) == "" {
			continue
		}
		pos := s.Position
		if pos < 1 {
			pos = 1
		}
		if pos > 99 {
			pos = 99
		}
		ordered = append(ordered, ArticleAdSlot{Position: pos, Content: s.Content})
	}
	if len(ordered) == 0 {
		return bodyHTML
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Position < ordered[j].Position })

	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_tirtayasa_post_root">` + bodyHTML + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return bodyHTML
	}
	root := doc.Find("#_tirtayasa_post_root")
	// Prefer langsung `<p>` di bawah wrapper artikel (struktur sederhana). Jika kosong —
	// pola umum editor: satu `<div>` membungkus paragraf — pakai descendant `p` agar inject
	// tidak hanya menyisip satu blok di akhir tanpa pola posisi yang masuk akal.
	paragraphs := root.ChildrenFiltered("p")
	if paragraphs.Length() == 0 {
		paragraphs = root.Find("p")
	}
	total := paragraphs.Length()

	if total == 0 {
		// No top-level paragraphs — append every ad at the end.
		for i := range ordered {
			root.AppendHtml(articleAdMarker(i))
		}
	} else {
		// Group ads by destination index, then insert after each target paragraph.
		idxAds := make(map[int][]int, len(ordered))
		for i, ad := range ordered {
			idx := ad.Position * total / 100
			if idx < 0 {
				idx = 0
			}
			if idx >= total {
				idx = total - 1
			}
			idxAds[idx] = append(idxAds[idx], i)
		}
		for idx, adIdxs := range idxAds {
			target := paragraphs.Eq(idx)
			// Append in reverse so the visual order matches the configured order at the same index.
			for j := len(adIdxs) - 1; j >= 0; j-- {
				target.AfterHtml(articleAdMarker(adIdxs[j]))
			}
		}
	}

	out, err := root.Html()
	if err != nil {
		return bodyHTML
	}

	// Replace markers with the actual ad markup (untouched script tags etc.).
	return articleAdMarkerRe.ReplaceAllStringFunc(out, func(match string) string {
		groups := articleAdMarkerRe.FindStringSubmatch(match)
		if len(groups) < 2 {
			return ""
		}
		var idx int
		_, err := fmt.Sscanf(groups[1], "%d", &idx)
		if err != nil || idx < 0 || idx >= len(ordered) {
			return ""
		}
		return `<div class="site-ad site-ad--center site-ad--inarticle" aria-label="Iklan">` + ordered[idx].Content + `</div>`
	})
}

func articleAdMarker(idx int) string {
	return fmt.Sprintf("<!--%s%d-->", articleAdMarkerPrefix, idx)
}
