package portfolio

import "testing"

func TestTemplates(t *testing.T) {
	all := Templates()
	if len(all) != 6 {
		t.Fatalf("got %d templates", len(all))
	}
	home := HomeTemplates()
	if len(home) != 6 {
		t.Fatalf("home got %d", len(home))
	}
	item, ok := TemplateBySlug("galeri")
	if !ok || item.Name != "Galeri" {
		t.Fatalf("%+v %v", item, ok)
	}
	if _, ok := TemplateBySlug("tidak-ada"); ok {
		t.Fatal("expected missing slug")
	}
	if NormalizeTemplate(" Studio ") != "studio" {
		t.Fatal(NormalizeTemplate(" Studio "))
	}
	if NormalizeTemplate("tidak-ada") != "" {
		t.Fatal("unknown template should be empty")
	}
	if ThemeClass("studio") != "pf-body-studio" || ThemeClass("") != "pf-body-klasik" {
		t.Fatal(ThemeClass("studio"), ThemeClass(""))
	}
	galeri := SamplePortfolio("galeri")
	klasik := SamplePortfolio("klasik")
	if galeri["Template"] != "galeri" || galeri["DisplayName"] == "" {
		t.Fatalf("%v", galeri["Template"])
	}
	if galeri["DisplayName"] == klasik["DisplayName"] {
		t.Fatal("each template preview should show a different person")
	}
}
