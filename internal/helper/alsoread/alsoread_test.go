package alsoread

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

func TestStripFrozen(t *testing.T) {
	in := `<p>a</p><aside class="blog-also-read" ` + DataAttr + `="1"><div class="blog-also-read-inner">x</div></aside><p>b</p>`
	out := Strip(in)
	if strings.Contains(out, "blog-also-read") {
		t.Fatalf("expected frozen aside removed, got: %q", out)
	}
	if !strings.Contains(out, "<p>a</p>") || !strings.Contains(out, "<p>b</p>") {
		t.Fatalf("expected paragraphs kept: %q", out)
	}
}

func TestInjectFrozenEveryThirdP(t *testing.T) {
	html := "<p>one</p><p>two</p><p>three</p><p>four</p><p>five</p><p>six</p>"
	rel := []entity.Post{
		{ID: 1, Title: "A", Slug: "a", Category: entity.Category{ID: 2, Slug: "cat"}},
		{ID: 2, Title: "B", Slug: "b", Category: entity.Category{ID: 2, Slug: "cat"}},
	}
	out := InjectFrozen(html, entity.ArticleSlugModeDirect, rel, entity.DefaultAlsoReadMaxInserts)
	if strings.Count(out, DataAttr+`="1"`) != 2 {
		t.Fatalf("expected 2 frozen blocks after 3rd and 6th p, got %d: %s", strings.Count(out, DataAttr+`="1"`), out)
	}
	if !strings.Contains(out, `href="/a"`) || !strings.Contains(out, `href="/b"`) {
		t.Fatalf("expected hrefs: %s", out)
	}
}

func TestInjectFrozenDefersToAfterTable(t *testing.T) {
	html := "<p>1</p><p>2</p><p>Intro table follows</p><table><tbody><tr><td>x</td></tr></tbody></table><p>4</p><p>5</p><p>six</p>"
	rel := []entity.Post{
		{ID: 1, Title: "A", Slug: "a", Category: entity.Category{ID: 2, Slug: "cat"}},
		{ID: 2, Title: "B", Slug: "b", Category: entity.Category{ID: 2, Slug: "cat"}},
	}
	out := InjectFrozen(html, entity.ArticleSlugModeDirect, rel, entity.DefaultAlsoReadMaxInserts)
	if strings.Count(out, DataAttr+`="1"`) != 2 {
		t.Fatalf("expected 2 frozen blocks, got %d: %s", strings.Count(out, DataAttr+`="1"`), out)
	}
	iTable := strings.Index(out, "</table>")
	iAside := strings.Index(out, "blog-also-read")
	if iTable < 0 || iAside < 0 || iAside < iTable {
		t.Fatalf("expected first Baca juga after table, table=%d aside=%d out=%s", iTable, iAside, out)
	}
	if !strings.Contains(out, `href="/a"`) || !strings.Contains(out, `href="/b"`) {
		t.Fatalf("expected hrefs: %s", out)
	}
}

func TestInjectFrozenDefersToAfterOl(t *testing.T) {
	html := "<p>a</p><p>b</p><p>Steps below</p><ol><li>one</li></ol><p>d</p><p>e</p><p>f</p>"
	rel := []entity.Post{
		{ID: 1, Title: "A", Slug: "a", Category: entity.Category{ID: 2, Slug: "cat"}},
		{ID: 2, Title: "B", Slug: "b", Category: entity.Category{ID: 2, Slug: "cat"}},
	}
	out := InjectFrozen(html, entity.ArticleSlugModeDirect, rel, entity.DefaultAlsoReadMaxInserts)
	if strings.Count(out, DataAttr+`="1"`) != 2 {
		t.Fatalf("expected 2 frozen blocks, got %d: %s", strings.Count(out, DataAttr+`="1"`), out)
	}
	iOl := strings.Index(out, "</ol>")
	iAside := strings.Index(out, "blog-also-read")
	if iOl < 0 || iAside < 0 || iAside < iOl {
		t.Fatalf("expected first Baca juga after ol, ol=%d aside=%d", iOl, iAside)
	}
}

func TestInjectFrozenDefersWrappedTable(t *testing.T) {
	html := "<p>x</p><p>y</p><p>z</p><div class=\"wrap\"><table><tr><td>t</td></tr></table></div><p>4</p><p>5</p><p>6</p>"
	rel := []entity.Post{
		{ID: 1, Title: "A", Slug: "a", Category: entity.Category{ID: 2, Slug: "cat"}},
		{ID: 2, Title: "B", Slug: "b", Category: entity.Category{ID: 2, Slug: "cat"}},
	}
	out := InjectFrozen(html, entity.ArticleSlugModeDirect, rel, entity.DefaultAlsoReadMaxInserts)
	if strings.Count(out, DataAttr+`="1"`) != 2 {
		t.Fatalf("expected 2 frozen blocks, got %d: %s", strings.Count(out, DataAttr+`="1"`), out)
	}
	iDiv := strings.Index(out, "</div>")
	iAside := strings.Index(out, "blog-also-read")
	if iDiv < 0 || iAside < 0 || iAside < iDiv {
		t.Fatalf("expected first Baca juga after wrapper div, div=%d aside=%d", iDiv, iAside)
	}
}

func TestRelocateLegacyAlsoReadBeforeTable(t *testing.T) {
	frag := `<p>Intro</p><aside class="blog-also-read" ` + DataAttr + `="1"><div class="inner"><a href="/x">X</a></div></aside><table><tr><td>1</td></tr></table>`
	wrapped := `<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><div id="_tirtayasa_post_root">` + frag + `</div></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		t.Fatal(err)
	}
	root := doc.Find("#_tirtayasa_post_root").First()
	relocateAlsoReadAfterFollowingBlocks(root)
	out, err := root.Html()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(out, "</table>") > strings.Index(out, "blog-also-read") {
		t.Fatalf("expected aside after table: %s", out)
	}
}
