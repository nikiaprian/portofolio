package titlesim

import (
	"sort"
	"strings"
	"unicode"
)

// DefaultThreshold is the minimum similarity ratio (0–1) for near-duplicate titles.
const DefaultThreshold = 0.9

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
