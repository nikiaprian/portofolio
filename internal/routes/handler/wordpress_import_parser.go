package handler

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// WpUser represents a parsed WordPress wp_users row.
type WpUser struct {
	ID             int
	Login          string
	Email          string
	Password       string
	UserRegistered string
}

// WpPost represents a parsed WordPress wp_posts row (post or page).
type WpPost struct {
	ID            int
	PostAuthor    int
	PostDate      string
	PostContent   string
	PostTitle     string
	PostExcerpt   string
	PostName      string
	PostStatus    string
	PostType      string
	Guid          string
	PostModified  string
	CommentStatus string
}

// WpTerm represents a parsed WordPress wp_terms row.
type WpTerm struct {
	TermID    int
	Name      string
	Slug      string
	TermGroup int
}

// WpTermTaxonomy represents a parsed WordPress wp_term_taxonomy row.
type WpTermTaxonomy struct {
	TermTaxonomyID int
	TermID         int
	Taxonomy       string
	Parent         int
	Count          int
}

// WpTermRelationship represents a parsed WordPress wp_term_relationships row.
type WpTermRelationship struct {
	ObjectID       int
	TermTaxonomyID int
}

// WpUserMeta represents a parsed WordPress wp_usermeta row.
type WpUserMeta struct {
	UserID    int
	MetaKey   string
	MetaValue string
}

// WpPostMeta represents a parsed WordPress wp_postmeta row.
type WpPostMeta struct {
	PostID    int
	MetaKey   string
	MetaValue string
}

// WpTermMeta represents a parsed WordPress wp_termmeta row.
type WpTermMeta struct {
	TermID    int
	MetaKey   string
	MetaValue string
}

// WpParsedData holds all parsed WordPress data.
type WpParsedData struct {
	Users             []WpUser
	UserMeta          []WpUserMeta
	Posts             []WpPost
	PostMeta          []WpPostMeta
	Terms             []WpTerm
	TermTaxonomies    []WpTermTaxonomy
	TermRelationships []WpTermRelationship
	TermMeta          []WpTermMeta
}

var wpTableSuffixes = []string{
	"users", "usermeta", "posts", "postmeta",
	"terms", "term_taxonomy", "term_relationships",
	"termmeta",
}

func parseLog(logs *[]string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[wp-import] %s", msg)
	if logs != nil {
		*logs = append(*logs, msg)
	}
}

// ParseWpSQL parses a WordPress SQL dump and extracts relevant tables.
func ParseWpSQL(sql string, logs *[]string) *WpParsedData {
	data := &WpParsedData{}

	prefix := detectWPPrefix(sql)
	parseLog(logs, "Table prefix: %s", prefix)

	tables := map[string]func(string){
		prefix + "users":              func(s string) { data.Users = append(data.Users, parseWpUsers(s, logs)...) },
		prefix + "usermeta":           func(s string) { data.UserMeta = append(data.UserMeta, parseWpUserMeta(s, logs)...) },
		prefix + "posts":              func(s string) { data.Posts = append(data.Posts, parseWpPosts(s, logs)...) },
		prefix + "postmeta":           func(s string) { data.PostMeta = append(data.PostMeta, parseWpPostMeta(s, logs)...) },
		prefix + "terms":              func(s string) { data.Terms = append(data.Terms, parseWpTerms(s, logs)...) },
		prefix + "termmeta":           func(s string) { data.TermMeta = append(data.TermMeta, parseWpTermMeta(s, logs)...) },
		prefix + "term_taxonomy":      func(s string) { data.TermTaxonomies = append(data.TermTaxonomies, parseWpTermTaxonomies(s, logs)...) },
		prefix + "term_relationships": func(s string) { data.TermRelationships = append(data.TermRelationships, parseWpTermRelationships(s, logs)...) },
	}

	insertRe := regexp.MustCompile(`(?i)(?:INSERT|REPLACE)\s+INTO\s+` + "`" + `?(\w+)` + "`" + `?\s*\(`)

	var stmts []struct {
		table string
		start int
	}

	allMatches := insertRe.FindAllStringSubmatchIndex(sql, -1)
	if allMatches == nil {
		insertRe2 := regexp.MustCompile(`(?i)(?:INSERT|REPLACE)\s+INTO\s+` + "`" + `?(\w+)` + "`" + `?\s+VALUES`)
		allMatches = insertRe2.FindAllStringSubmatchIndex(sql, -1)
	}
	if allMatches == nil {
		parseLog(logs, "Tidak ditemukan INSERT/REPLACE INTO statements")
		return data
	}

	for _, m := range allMatches {
		if len(m) < 4 {
			continue
		}
		tableName := strings.ToLower(strings.Trim(sql[m[2]:m[3]], "`"))
		stmts = append(stmts, struct {
			table string
			start int
		}{table: tableName, start: m[0]})
	}

	parseLog(logs, "Ditemukan %d INSERT/REPLACE statements", len(stmts))

	for i, stmt := range stmts {
		if parser, ok := tables[stmt.table]; ok {
			end := len(sql)
			if i+1 < len(stmts) {
				end = stmts[i+1].start
			}
			parser(sql[stmt.start:end])
		}
	}

	parseLog(logs, "Hasil: %d users, %d usermeta, %d posts, %d postmeta, %d terms, %d term_taxonomy, %d term_relationships",
		len(data.Users), len(data.UserMeta), len(data.Posts), len(data.PostMeta),
		len(data.Terms), len(data.TermTaxonomies), len(data.TermRelationships))

	return data
}

func detectWPPrefix(sql string) string {
	lower := strings.ToLower(sql)

	// Check for CREATE TABLE statements
	if idx := strings.Index(lower, "create table"); idx >= 0 {
		rest := lower[idx:]
		// Try with and without backticks
		for _, trim := range []string{"create table `", "create table ", "create table\t"} {
			trimmed := strings.TrimPrefix(rest, trim)
			if trimmed != rest {
				endIdx := strings.IndexAny(trimmed, " `\n\r\t")
				if endIdx > 0 {
					tname := trimmed[:endIdx]
					for _, suffix := range wpTableSuffixes {
						if strings.HasSuffix(tname, suffix) {
							p := strings.TrimSuffix(tname, suffix)
							if p != "" {
								return p + "_"
							}
						}
					}
				}
			}
		}
	}

	// Check if wp_posts exists directly
	if strings.Contains(lower, "wp_posts") {
		return "wp_"
	}

	// Find any table ending with _posts
	postIdx := strings.Index(lower, "_posts")
	if postIdx > 0 {
		start := postIdx - 1
		for start >= 0 {
			c := lower[start]
			if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
				start--
			} else {
				break
			}
		}
		prefix := lower[start+1 : postIdx+1]
		if prefix != "" {
			return prefix
		}
	}

	return "wp_"
}

func extractValueGroups(valuesSQL string) []string {
	upper := strings.ToUpper(valuesSQL)
	valIdx := strings.Index(upper, "VALUES")
	if valIdx < 0 {
		return nil
	}
	rest := valuesSQL[valIdx+6:]

	var groups []string
	var current strings.Builder
	depth := 0
	inString := false
	var prevChar byte

	for i := 0; i < len(rest); i++ {
		ch := rest[i]

		if inString {
			current.WriteByte(ch)
			if ch == '\\' && i+1 < len(rest) {
				nextCh := rest[i+1]
				if nextCh == '\'' || nextCh == '\\' || nextCh == '"' || nextCh == 'n' || nextCh == 'r' || nextCh == 't' || nextCh == '0' {
					current.WriteByte(nextCh)
					i++
					prevChar = nextCh
					continue
				}
			}
			if ch == '\'' && prevChar != '\\' {
				inString = false
			}
			prevChar = ch
			continue
		}

		if ch == '\'' {
			inString = true
			current.WriteByte(ch)
			prevChar = ch
			continue
		}

		if ch == '(' {
			depth++
			if depth == 1 {
				current.Reset()
				continue
			}
		}

		if ch == ')' {
			depth--
			if depth == 0 {
				groups = append(groups, current.String())
				current.Reset()
				for i+1 < len(rest) && (rest[i+1] == ',' || rest[i+1] == ' ' || rest[i+1] == '\n' || rest[i+1] == '\r' || rest[i+1] == '\t') {
					i++
				}
				continue
			}
		}

		if depth > 0 {
			current.WriteByte(ch)
		}
		prevChar = ch
	}

	return groups
}

func parseRowValues(group string) []string {
	var fields []string
	var current strings.Builder
	inString := false
	var prevChar byte

	for i := 0; i < len(group); i++ {
		ch := group[i]

		if inString {
			if ch == '\\' && i+1 < len(group) {
				next := group[i+1]
				switch next {
				case '\'':
					current.WriteByte('\'')
					i++
					prevChar = '\''
					continue
				case '\\':
					current.WriteByte('\\')
					i++
					prevChar = '\\'
					continue
				case '"':
					current.WriteByte('"')
					i++
					prevChar = '"'
					continue
				case 'n':
					current.WriteByte('\n')
					i++
					prevChar = 'n'
					continue
				case 'r':
					current.WriteByte('\r')
					i++
					prevChar = 'r'
					continue
				case 't':
					current.WriteByte('\t')
					i++
					prevChar = 't'
					continue
				case '0':
					current.WriteByte(0)
					i++
					prevChar = '0'
					continue
				default:
					current.WriteByte(ch)
					prevChar = ch
					continue
				}
			}
			if ch == '\'' && prevChar != '\\' {
				inString = false
				continue
			}
			current.WriteByte(ch)
			prevChar = ch
			continue
		}

		if ch == '\'' {
			inString = true
			current.Reset()
			prevChar = ch
			continue
		}

		if ch == ',' {
			val := strings.TrimSpace(current.String())
			fields = append(fields, val)
			current.Reset()
			prevChar = ch
			continue
		}

		current.WriteByte(ch)
		prevChar = ch
	}

	if current.Len() > 0 {
		val := strings.TrimSpace(current.String())
		fields = append(fields, val)
	}

	return fields
}

func parseWpUsers(sql string, logs *[]string) []WpUser {
	groups := extractValueGroups(sql)
	if len(groups) == 0 {
		parseLog(logs, "wp_users: tidak ada value groups ditemukan")
	}
	var users []WpUser
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 7 {
			continue
		}
		u := WpUser{
			ID:             atoi(fields[0]),
			Login:          unquote(fields[1]),
			Email:          unquote(fields[4]),
			Password:       unquote(fields[2]),
			UserRegistered: unquote(fields[6]),
		}
		if u.ID > 0 {
			users = append(users, u)
		}
	}
	parseLog(logs, "wp_users: %d rows parsed", len(users))
	return users
}

func parseWpUserMeta(sql string, logs *[]string) []WpUserMeta {
	groups := extractValueGroups(sql)
	var metas []WpUserMeta
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 4 {
			continue
		}
		m := WpUserMeta{
			UserID:    atoi(fields[1]),
			MetaKey:   unquote(fields[2]),
			MetaValue: unquote(fields[3]),
		}
		if m.UserID > 0 {
			metas = append(metas, m)
		}
	}
	parseLog(logs, "wp_usermeta: %d rows parsed", len(metas))
	return metas
}

func parseWpPosts(sql string, logs *[]string) []WpPost {
	groups := extractValueGroups(sql)
	if len(groups) == 0 {
		parseLog(logs, "wp_posts: tidak ada value groups ditemukan")
	}
	var posts []WpPost
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 21 {
			continue
		}
		p := WpPost{
			ID:            atoi(fields[0]),
			PostAuthor:    atoi(fields[1]),
			PostDate:      unquote(fields[2]),
			PostContent:   fields[4],
			PostTitle:     unquote(fields[5]),
			PostExcerpt:   unquote(fields[6]),
			PostName:      unquote(fields[11]),
			PostStatus:    unquote(fields[7]),
			PostType:      unquote(fields[20]),
			Guid:          unquote(fields[18]),
			PostModified:  unquote(fields[14]),
			CommentStatus: unquote(fields[8]),
		}
		if p.ID > 0 && (p.PostType == "post" || p.PostType == "page" || p.PostType == "attachment") {
			posts = append(posts, p)
		}
	}
	parseLog(logs, "wp_posts: %d rows parsed (post/page/attachment)", len(posts))
	return posts
}

func parseWpPostMeta(sql string, logs *[]string) []WpPostMeta {
	groups := extractValueGroups(sql)
	var metas []WpPostMeta
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 4 {
			continue
		}
		m := WpPostMeta{
			PostID:    atoi(fields[1]),
			MetaKey:   unquote(fields[2]),
			MetaValue: unquote(fields[3]),
		}
		if m.PostID > 0 {
			metas = append(metas, m)
		}
	}
	parseLog(logs, "wp_postmeta: %d rows parsed", len(metas))
	return metas
}

func parseWpTermMeta(sql string, logs *[]string) []WpTermMeta {
	groups := extractValueGroups(sql)
	var metas []WpTermMeta
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 4 {
			continue
		}
		m := WpTermMeta{
			TermID:    atoi(fields[1]),
			MetaKey:   unquote(fields[2]),
			MetaValue: unquote(fields[3]),
		}
		if m.TermID > 0 {
			metas = append(metas, m)
		}
	}
	parseLog(logs, "wp_termmeta: %d rows parsed", len(metas))
	return metas
}

func parseWpTerms(sql string, logs *[]string) []WpTerm {
	groups := extractValueGroups(sql)
	if len(groups) == 0 {
		parseLog(logs, "wp_terms: tidak ada value groups ditemukan")
	}
	var terms []WpTerm
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 4 {
			continue
		}
		t := WpTerm{
			TermID:    atoi(fields[0]),
			Name:      unquote(fields[1]),
			Slug:      unquote(fields[2]),
			TermGroup: atoi(fields[3]),
		}
		if t.TermID > 0 {
			terms = append(terms, t)
		}
	}
	parseLog(logs, "wp_terms: %d rows parsed", len(terms))
	return terms
}

func parseWpTermTaxonomies(sql string, logs *[]string) []WpTermTaxonomy {
	groups := extractValueGroups(sql)
	var taxa []WpTermTaxonomy
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 5 {
			continue
		}
		t := WpTermTaxonomy{
			TermTaxonomyID: atoi(fields[0]),
			TermID:         atoi(fields[1]),
			Taxonomy:       unquote(fields[2]),
			Parent:         atoi(fields[3]),
			Count:          atoi(fields[4]),
		}
		if t.TermTaxonomyID > 0 {
			taxa = append(taxa, t)
		}
	}
	parseLog(logs, "wp_term_taxonomy: %d rows parsed", len(taxa))
	return taxa
}

func parseWpTermRelationships(sql string, logs *[]string) []WpTermRelationship {
	groups := extractValueGroups(sql)
	var rels []WpTermRelationship
	for _, g := range groups {
		fields := parseRowValues(g)
		if len(fields) < 2 {
			continue
		}
		r := WpTermRelationship{
			ObjectID:       atoi(fields[0]),
			TermTaxonomyID: atoi(fields[1]),
		}
		if r.ObjectID > 0 {
			rels = append(rels, r)
		}
	}
	parseLog(logs, "wp_term_relationships: %d rows parsed", len(rels))
	return rels
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "NULL" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	if len(s) > 0 && s[0] == '-' {
		n = -n
	}
	return n
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if s == "NULL" {
		return ""
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
	}
	s = strings.ReplaceAll(s, "\\'", "'")
	s = strings.ReplaceAll(s, "\\\"", "\"")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\0", "\x00")
	return s
}
