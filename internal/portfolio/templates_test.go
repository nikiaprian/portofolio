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
}
