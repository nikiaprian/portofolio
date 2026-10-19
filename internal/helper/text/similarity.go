package text

import (
	"sort"
	"strings"
	"unicode"
)

// DefaultThreshold is the minimum similarity ratio (0–1) for near-duplicate titles.
const DefaultThreshold = 0.9

// BulkTitleThreshold is used when checking bulk-import titles against existing posts.
const BulkTitleThreshold = 0.5

// Normalize collapses whitespace and lowercases for comparison.
func Normalize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.TrimSpace(b.String())
}

// Ratio returns similarity in [0,1] using normalized Levenshtein distance.
func Ratio(a, b string) float64 {
	a = Normalize(a)
	b = Normalize(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	ar := []rune(a)
	br := []rune(b)
	dist := levenshtein(ar, br)
	maxLen := len(ar)
	if len(br) > maxLen {
		maxLen = len(br)
	}
	if maxLen == 0 {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

// IsSimilar reports whether a and b meet or exceed threshold (e.g. 0.9).
func IsSimilar(a, b string, threshold float64) bool {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	return Ratio(a, b) >= threshold
}

// MatchScoreForAutoHref scores how well a content phrase matches a link candidate title.
// Deprecated for article autohref — use MatchScoreForAutoHrefArticle instead.
func MatchScoreForAutoHref(phrase, title string) float64 {
	return MatchScoreForAutoHrefArticle(phrase, title)
}

const (
	// AutoHrefArticleMinWords skips single-word false positives (e.g. "BPJS").
	AutoHrefArticleMinWords = 3
	// AutoHrefArticleRatioThreshold applies when the span is not a consecutive phrase in the title.
	AutoHrefArticleRatioThreshold = 0.7
	// AutoHrefArticlePhraseThreshold applies when the span appears word-for-word inside the title.
	AutoHrefArticlePhraseThreshold = 0.35
)

// MatchScoreForAutoHrefArticle returns a realistic score for in-body article linking.
func MatchScoreForAutoHrefArticle(phrase, title string) float64 {
	phraseWords := strings.Fields(Normalize(phrase))
	if len(phraseWords) >= AutoHrefArticleMinWords && ConsecutivePhraseInTitle(phrase, title) {
		return phraseWordCoverageScore(phrase, title)
	}
	return Ratio(phrase, title)
}

// QualifiesAutoHrefArticle reports whether a content span should link to an article title.
func QualifiesAutoHrefArticle(phrase, title string) bool {
	phraseWords := strings.Fields(Normalize(phrase))
	if len(phraseWords) < AutoHrefArticleMinWords {
		return false
	}
	score := MatchScoreForAutoHrefArticle(phrase, title)
	if ConsecutivePhraseInTitle(phrase, title) {
		return score >= AutoHrefArticlePhraseThreshold
	}
	return score >= AutoHrefArticleRatioThreshold
}

// MatchScore compares two titles for bulk duplicate detection.
// Partial phrase matches return word-coverage (shorter word count / longer word count), not 1.0.
func MatchScore(a, b string) float64 {
	aNorm := Normalize(a)
	bNorm := Normalize(b)
	if aNorm == bNorm {
		return 1.0
	}
	ratio := Ratio(a, b)
	if cov := phraseWordCoverageScore(a, b); cov > ratio {
		return cov
	}
	return ratio
}

func phraseWordCoverageScore(a, b string) float64 {
	aWords := strings.Fields(Normalize(a))
	bWords := strings.Fields(Normalize(b))
	if len(aWords) == 0 || len(bWords) == 0 {
		return 0
	}
	if ConsecutivePhraseInTitle(a, b) {
		return float64(len(aWords)) / float64(len(bWords))
	}
	if ConsecutivePhraseInTitle(b, a) {
		return float64(len(bWords)) / float64(len(aWords))
	}
	return 0
}

// ConsecutivePhraseInTitle reports whether all phrase words appear in order in title.
func ConsecutivePhraseInTitle(phrase, title string) bool {
	phraseWords := strings.Fields(Normalize(phrase))
	titleWords := strings.Fields(Normalize(title))
	if len(phraseWords) == 0 || len(phraseWords) > len(titleWords) {
		return false
	}
	for i := 0; i <= len(titleWords)-len(phraseWords); i++ {
		if wordsMatchSequence(phraseWords, titleWords[i:i+len(phraseWords)]) {
			return true
		}
	}
	return false
}

func wordsMatchSequence(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && !WordSimilar(a[i], b[i]) {
			return false
		}
	}
	return true
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = del
			if ins < cur[j] {
				cur[j] = ins
			}
			if sub < cur[j] {
				cur[j] = sub
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// FuzzyRatio returns a robust similarity score in [0,1] that handles word reordering and word-level differences.
func FuzzyRatio(a, b string) float64 {
	a = Normalize(a)
	b = Normalize(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}

	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	// Calculate word-level ratio
	sortedA := make([]string, len(wordsA))
	copy(sortedA, wordsA)
	sort.Strings(sortedA)

	sortedB := make([]string, len(wordsB))
	copy(sortedB, wordsB)
	sort.Strings(sortedB)

	dp := make([][]int, len(sortedA)+1)
	for i := range dp {
		dp[i] = make([]int, len(sortedB)+1)
	}
	for i := 0; i <= len(sortedA); i++ {
		dp[i][0] = i
	}
	for j := 0; j <= len(sortedB); j++ {
		dp[0][j] = j
	}
	for i := 1; i <= len(sortedA); i++ {
		for j := 1; j <= len(sortedB); j++ {
			cost := 1
			if WordSimilar(sortedA[i-1], sortedB[j-1]) {
				cost = 0
			}
			del := dp[i-1][j] + 1
			ins := dp[i][j-1] + 1
			sub := dp[i-1][j-1] + cost
			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			dp[i][j] = min
		}
	}
	wordDist := dp[len(sortedA)][len(sortedB)]
	maxWordLen := len(sortedA)
	if len(sortedB) > maxWordLen {
		maxWordLen = len(sortedB)
	}
	wordRatio := 1.0
	if maxWordLen > 0 {
		wordRatio = 1.0 - float64(wordDist)/float64(maxWordLen)
	}

	// Compute character-level ratio on sorted-joined words
	sortedJoinedA := strings.Join(sortedA, " ")
	sortedJoinedB := strings.Join(sortedB, " ")
	charRatio := Ratio(sortedJoinedA, sortedJoinedB)

	// Return the minimum of charRatio and wordRatio to ensure both aspects match
	if charRatio < wordRatio {
		return charRatio
	}
	return wordRatio
}

func WordSimilar(w1, w2 string) bool {
	if w1 == w2 {
		return true
	}
	return Ratio(w1, w2) >= 0.6
}
