package posthtml

import (
	"strings"
	"testing"
)

func TestSanitizePublicArticleHTML_StripsChatGPTChromeAndOrphanCloses(t *testing.T) {
	raw := `
<p data-start="1" data-end="2">Intro aman.</p>
<div class="TyagGW_tableContainer">
  <div class="group TyagGW_tableWrapper flex flex-col-reverse w-fit" tabindex="-1">
    <table><tbody><tr><th>Keterangan</th><th>Harga</th></tr><tr><td>Jual</td><td>100</td></tr></tbody></table>
  </div>
</div>
<section class="text-token-text-primary R6Vx5W_threadScrollVars" data-turn="assistant" data-testid="conversation-turn-1">
  <div data-conversation-screenshot-content="">
    <h2 data-section-id="x">Judul Bagian</h2>
    <p>Konten tetap ada.</p>
  </div>
</section>
<div class="mt-3 w-full empty:hidden">&nbsp;</div>
</div></div></section></div></div>
`
	got := SanitizePublicArticleHTML(raw)
	if got == "" {
		t.Fatal("expected sanitized html")
	}
	if strings.Contains(got, "TyagGW_") {
		t.Fatalf("table chrome class leaked: %s", got)
	}
	if strings.Contains(got, "conversation-screenshot") || strings.Contains(got, "data-turn") {
		t.Fatalf("chatgpt attrs leaked: %s", got)
	}
	if strings.Contains(got, "empty:hidden") {
		t.Fatalf("empty chrome spacer leaked: %s", got)
	}
	if !strings.Contains(got, "<table") || !strings.Contains(got, "Keterangan") {
		t.Fatalf("table content missing: %s", got)
	}
	if !strings.Contains(got, "Judul Bagian") || !strings.Contains(got, "Konten tetap ada") {
		t.Fatalf("article text missing: %s", got)
	}
	// Orphan closes must not survive serialization.
	if strings.HasSuffix(strings.TrimSpace(got), "</div>") && strings.Count(got, "<div") < strings.Count(got, "</div>") {
		t.Fatalf("unbalanced divs after sanitize: %s", got)
	}
	if strings.Count(got, "<div") != strings.Count(got, "</div>") {
		t.Fatalf("div mismatch open=%d close=%d html=%s", strings.Count(got, "<div"), strings.Count(got, "</div>"), got)
	}
}

func TestSanitizePublicArticleHTML_KeepsCleanArticle(t *testing.T) {
	raw := `<p>Halo</p><h2>Judul</h2><p>Isi <strong>tebal</strong>.</p>`
	got := SanitizePublicArticleHTML(raw)
	if !strings.Contains(got, "Halo") || !strings.Contains(got, "Judul") || !strings.Contains(got, "tebal") {
		t.Fatalf("clean article changed unexpectedly: %s", got)
	}
}

func TestSanitizePublicArticleHTML_UnwrapsScrollSentinelWithContent(t *testing.T) {
	raw := `
<div class="pointer-events-none -mt-px h-px" aria-hidden="true">
  <h2>Judul Tetap Ada</h2>
  <p>Paragraf di dalam sentinel.</p>
</div>
<div class="pointer-events-none -mt-px h-px" aria-hidden="true"></div>
`
	got := SanitizePublicArticleHTML(raw)
	if !strings.Contains(got, "Judul Tetap Ada") || !strings.Contains(got, "Paragraf di dalam sentinel") {
		t.Fatalf("content inside scroll sentinel was dropped: %s", got)
	}
	if strings.Contains(got, "pointer-events-none") {
		t.Fatalf("sentinel wrapper should be unwrapped: %s", got)
	}
}

func TestSanitizePublicArticleHTML_StripsPasteBloat(t *testing.T) {
	raw := `<style>.lst-kix_abc{font-size:11pt}` + strings.Repeat("x", 80000) + `</style><!--[if gte mso 9]><xml></xml><![endif]--><p class="MsoNormal">Halo dunia, artikel pendek.</p><img src="data:image/png;base64,aaa" alt="x">`
	got := SanitizePublicArticleHTML(raw)
	if strings.Contains(got, "<style") || strings.Contains(got, "data:image") || strings.Contains(got, "MsoNormal") {
		t.Fatalf("paste bloat leaked: %s", got)
	}
	if !strings.Contains(got, "Halo dunia") {
		t.Fatalf("visible text lost: %s", got)
	}
	if len(got) > 400 {
		t.Fatalf("cleaned HTML still large (%d): %s", len(got), got)
	}
}

func TestSanitizePublicArticleHTML_StripsIsSelectedEnd(t *testing.T) {
	raw := `<p class="isSelectedEnd">Paragraf dari editor.</p><p class="isSelectedEnd foo">Lainnya</p>`
	got := SanitizePublicArticleHTML(raw)
	if strings.Contains(got, "isSelectedEnd") {
		t.Fatalf("isSelectedEnd should be stripped: %s", got)
	}
	if !strings.Contains(got, "Paragraf dari editor") || !strings.Contains(got, "Lainnya") {
		t.Fatalf("text missing after strip: %s", got)
	}
	if !strings.Contains(got, `class="foo"`) {
		t.Fatalf("non-junk class should remain: %s", got)
	}
}
