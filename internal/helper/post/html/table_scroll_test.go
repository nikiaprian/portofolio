package posthtml

import (
	"strings"
	"testing"
)

func TestWrapTablesForMobileScroll(t *testing.T) {
	raw := `<p>Intro</p><table><tbody><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></tbody></table><p>Outro</p>`
	got := WrapTablesForMobileScroll(raw)
	if !strings.Contains(got, `class="blog-table-scroll"`) {
		t.Fatalf("expected scroll wrapper: %s", got)
	}
	if !strings.Contains(got, "<table>") || !strings.Contains(got, "Intro") || !strings.Contains(got, "Outro") {
		t.Fatalf("table/content missing: %s", got)
	}
	// Idempotent
	again := WrapTablesForMobileScroll(got)
	if strings.Count(again, `blog-table-scroll`) != 1 {
		t.Fatalf("expected single wrapper after second wrap: %s", again)
	}
}
