package handler

import "strings"

// logSearchLike returns a SQL LIKE "contains" pattern. Wildcards in the raw query are
// stripped so user input cannot broaden the match beyond substring search.
func logSearchLike(raw string) (like string, ok bool) {
	q := strings.TrimSpace(raw)
	if q == "" {
		return "", false
	}
	q = strings.ReplaceAll(q, "%", "")
	q = strings.ReplaceAll(q, "_", "")
	q = strings.TrimSpace(q)
	if q == "" {
		return "", false
	}
	return "%" + q + "%", true
}
