package posthtml

import (
	"strings"
	"testing"
)

func TestSanitizeAIDraftHTML_ConvertsMarkdownAndStripsPlan(t *testing.T) {
	raw := `Plan mode aktif — saya tidak akan menulis artikel dulu, hanya menyusun rencana.

## Rencana Artikel

**Judul:** Franco Baresi

Kalau rencana ini sudah sesuai, keluar dari plan mode.

## Siapa Franco Baresi?

Franco Baresi adalah legenda AC Milan.

### Karier

Ia bermain selama 20 musim.`

	got := SanitizeAIDraftHTML(raw)
	if strings.Contains(strings.ToLower(got), "plan mode") {
		t.Fatalf("plan chatter still present: %q", got)
	}
	if strings.Contains(got, "## ") {
		t.Fatalf("markdown headings left: %q", got)
	}
	if !strings.Contains(got, "<h2>") && !strings.Contains(got, "<h3>") {
		t.Fatalf("expected HTML headings, got: %q", got)
	}
	if !strings.Contains(got, "Franco Baresi adalah") {
		t.Fatalf("expected article body, got: %q", got)
	}
}

func TestSanitizeAIDraftHTML_StripsFences(t *testing.T) {
	raw := "```html\n<p>Lead</p><h2>Topik</h2><p>Isi</p>\n```"
	got := SanitizeAIDraftHTML(raw)
	if strings.Contains(got, "```") {
		t.Fatalf("fence left: %q", got)
	}
	if !strings.Contains(got, "<p>") {
		t.Fatalf("expected html: %q", got)
	}
}

func TestLooksLikeAIPlanOnly(t *testing.T) {
	plan := `Plan mode aktif — saya tidak akan menulis artikel dulu.
## Rencana Artikel
1. Lead
2. FAQ
Kalau rencana ini sudah sesuai, keluar dari plan mode.`
	if !LooksLikeAIPlanOnly(plan) {
		t.Fatal("expected plan-only detection")
	}
	article := `<p>Lead tentang Franco Baresi.</p><h2>Siapa Franco Baresi?</h2><p>` + strings.Repeat("isi ", 80) + `</p>`
	if LooksLikeAIPlanOnly(article) {
		t.Fatal("clean HTML should not look like plan-only")
	}
}
