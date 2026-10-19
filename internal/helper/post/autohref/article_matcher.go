package autohref

import (
	"strings"

	titlesim "github.com/Bebii-Digital/bebii-blog/internal/helper/text"
)

const (
	autoHrefMaxPhraseWords       = 8
	maxAutoHrefArticleCandidates = 200
	maxTotalTitlePhrases         = 4000
	maxTitleWordsForPhraseGen    = 14
	minArticleLinkTextRunes      = 16
)

func normalizeMatchText(text string) string {
	return strings.ReplaceAll(text, "\u00a0", " ")
}

type titlePhrase struct {
	phrase string
	cand   hrefCandidate
}

type articleMatcher struct {
	exact  []hrefCandidate
	byLead map[string][]titlePhrase
}

func buildArticleMatcher(candidates []hrefCandidate) *articleMatcher {
	m := &articleMatcher{
		exact:  candidates,
		byLead: make(map[string][]titlePhrase),
	}
	phraseCount := 0
	add := func(phrase string, c hrefCandidate) {
		if phrase == "" || phraseCount >= maxTotalTitlePhrases {
			return
		}
		lead := strings.Fields(phrase)
		if len(lead) == 0 {
			return
		}
		m.byLead[lead[0]] = append(m.byLead[lead[0]], titlePhrase{phrase: phrase, cand: c})
		phraseCount++
	}

	for _, c := range candidates {
		if phraseCount >= maxTotalTitlePhrases {
			break
		}
		words := strings.Fields(titlesim.Normalize(c.text))
		if len(words) < autoHrefArticleMinWords {
			continue
		}
		if len(words) > maxTitleWordsForPhraseGen {
			add(strings.Join(words, " "), c)
			continue
		}
		maxI := len(words) - autoHrefArticleMinWords
		for i := 0; i <= maxI; i++ {
			maxWl := autoHrefMaxPhraseWords
			if rem := len(words) - i; rem < maxWl {
				maxWl = rem
			}
			for wl := autoHrefArticleMinWords; wl <= maxWl; wl++ {
				if phraseCount >= maxTotalTitlePhrases {
					return m
				}
				if float64(wl)/float64(len(words)) < titlesim.AutoHrefArticlePhraseThreshold {
					continue
				}
				add(strings.Join(words[i:i+wl], " "), c)
			}
		}
	}
	return m
}

func (m *articleMatcher) phrasesRelevant(text string) []titlePhrase {
	if m == nil || len(m.byLead) == 0 {
		return nil
	}
	words := strings.Fields(titlesim.Normalize(text))
	if len(words) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(words))
	var out []titlePhrase
	for _, w := range words {
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, m.byLead[w]...)
	}
	return out
}

func findNextArticleMatchAtCursor(text string, cursor int, m *articleMatcher, relevant []titlePhrase, usedArt map[string]struct{}) (matchResult, bool) {
	if m == nil || cursor >= len(text) {
		return matchResult{}, false
	}
	text = normalizeMatchText(text)
	lower := strings.ToLower(text)
	if cursor > len(lower) {
		return matchResult{}, false
	}

	var best matchResult
	found := false
	consider := func(start, end int, cand hrefCandidate) {
		if start < cursor || !hasWordBoundaries(text, start, end) {
			return
		}
		if articleURLUsed(usedArt, cand.url) {
			return
		}
		h := matchResult{start: start, end: end, cand: cand}
		if !found || h.start < best.start || (h.start == best.start && h.end > best.end) {
			best = h
			found = true
		}
	}

	tail := lower[cursor:]
	for _, c := range m.exact {
		target := strings.ToLower(strings.TrimSpace(c.text))
		if target == "" {
			continue
		}
		rel := strings.Index(tail, target)
		if rel < 0 {
			continue
		}
		i := cursor + rel
		consider(i, i+len(target), c)
	}

	for _, tp := range relevant {
		phrase := tp.phrase
		if phrase == "" {
			continue
		}
		rel := strings.Index(tail, phrase)
		if rel < 0 {
			continue
		}
		i := cursor + rel
		consider(i, i+len(phrase), tp.cand)
	}

	return best, found
}

func articleURLUsed(used map[string]struct{}, href string) bool {
	if used == nil {
		return false
	}
	key := articleURLKey(href)
	if key == "" {
		return false
	}
	_, ok := used[key]
	return ok
}
