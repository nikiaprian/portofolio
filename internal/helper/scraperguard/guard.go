package scraperguard

import (
	"bytes"
	"strings"
)

const (
	MetaName  = "tirtayasa-system-guard"
	MetaValue = "tirtayasa-system"
)

func MetaTagHTML() string {
	return `<meta name="` + MetaName + `" content="` + MetaValue + `" />`
}

func MarkerComment() string {
	return `<!-- ` + MetaName + `:` + MetaValue + ` -->`
}

func ContainsMarkerBytes(raw []byte) bool {
	return ContainsMarkerString(string(raw))
}

func ContainsMarkerString(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, strings.ToLower(MetaName)) && strings.Contains(low, strings.ToLower(MetaValue))
}

func PrependMarkerToHTML(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || ContainsMarkerString(s) {
		return s
	}
	return MarkerComment() + "\n" + s
}

func PrependMarkerToCDATA(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || ContainsMarkerString(s) {
		return s
	}
	var b bytes.Buffer
	b.WriteString(MarkerComment())
	b.WriteByte('\n')
	b.WriteString(s)
	return b.String()
}
