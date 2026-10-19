package seeds

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultAndTheme1Fragments(t *testing.T) {
	root := findProjectRoot()
	if !looksLikeProjectRoot(root) {
		t.Fatalf("project root not found from %q", root)
	}

	defaultDir := filepath.Join(root, "db", "seeds", "layouts", "default")
	defaultFrags, err := loadSiteLayoutFragmentsFromMap(defaultDir, siteLayoutDefaultFragmentFiles)
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	theme1Dir := filepath.Join(root, "db", "seeds", "layouts", "theme1")
	theme1Frags, err := loadSiteLayoutFragmentsFromMap(theme1Dir, siteLayoutTheme1FragmentFiles)
	if err != nil {
		t.Fatalf("load theme1: %v", err)
	}
	theme2Dir := filepath.Join(root, "db", "seeds", "layouts", "theme2")
	theme2Frags, err := loadSiteLayoutFragmentsFromMap(theme2Dir, siteLayoutTheme2FragmentFiles)
	if err != nil {
		t.Fatalf("load theme2: %v", err)
	}
	theme3Dir := filepath.Join(root, "db", "seeds", "layouts", "theme3")
	theme3Frags, err := loadSiteLayoutFragmentsFromMap(theme3Dir, siteLayoutTheme3FragmentFiles)
	if err != nil {
		t.Fatalf("load theme3: %v", err)
	}

	for key := range siteLayoutDefaultFragmentFiles {
		if strings.TrimSpace(defaultFrags[key]) == "" {
			t.Fatalf("default fragment %q empty", key)
		}
		if strings.TrimSpace(theme1Frags[key]) == "" {
			t.Fatalf("theme1 fragment %q empty", key)
		}
		if strings.TrimSpace(theme2Frags[key]) == "" {
			t.Fatalf("theme2 fragment %q empty", key)
		}
		if strings.TrimSpace(theme3Frags[key]) == "" {
			t.Fatalf("theme3 fragment %q empty", key)
		}
	}

	if strings.Contains(defaultFrags["home_body"], "theme1-home") {
		t.Fatalf("default home_body must not contain theme1-home")
	}
	if strings.Contains(defaultFrags["header"], "theme1-header") {
		t.Fatalf("default header must not contain theme1-header")
	}
	if !strings.Contains(defaultFrags["home_body"], "home-feature-carousel") {
		t.Fatalf("default home_body missing home-feature-carousel")
	}

	if !strings.Contains(theme1Frags["header"], "theme1-header") {
		t.Fatalf("theme1 header missing theme1-header marker")
	}
	if !strings.Contains(theme1Frags["header"], "initTheme1Header") {
		t.Fatalf("theme1 header missing renamed initTheme1Header function")
	}
	if strings.Contains(theme1Frags["header"], "> Galeri <") || strings.Contains(theme1Frags["header"], ">Galeri<") {
		t.Fatalf("theme1 header should not hardcode a Galeri nav item")
	}
	if !strings.Contains(theme1Frags["home_body"], "theme1-home") {
		t.Fatalf("theme1 home_body missing theme1-home marker")
	}
	if !strings.Contains(theme1Frags["home_body"], "Artikel Kegiatan") {
		t.Fatalf("theme1 home_body should rename Video Kegiatan to Artikel Kegiatan")
	}
	if strings.Contains(theme1Frags["home_body"], "Video Kegiatan") {
		t.Fatalf("theme1 home_body should not keep Video Kegiatan label")
	}
	if strings.Contains(strings.ToLower(theme1Frags["home_body"]), ">galeri<") || strings.Contains(theme1Frags["home_body"], "border_section\">Galeri") {
		t.Fatalf("theme1 home_body should omit Galeri section")
	}
	if !strings.Contains(theme1Frags["home_body"], "initTheme1HeroCarousel") {
		t.Fatalf("theme1 home_body missing renamed initTheme1HeroCarousel")
	}
	if !strings.Contains(theme1Frags["home_body"], "initTheme1PengumumanTicker") {
		t.Fatalf("theme1 home_body missing renamed initTheme1PengumumanTicker")
	}
	// Theme1 keeps the accent footer bar (distinct from theme2 gray editorial footer).
	if !strings.Contains(theme1Frags["footer"], "bg-accent") {
		t.Fatalf("theme1 footer should keep legacy bg-accent footer")
	}
	if !strings.Contains(theme1Frags["footer"], "theme1-footer") {
		t.Fatalf("theme1 footer missing theme1-footer marker")
	}
	if defaultFrags["header"] == theme1Frags["header"] {
		t.Fatalf("theme1 header should differ from default")
	}
	if defaultFrags["home_body"] == theme1Frags["home_body"] {
		t.Fatalf("theme1 home_body should differ from default")
	}

	if !strings.Contains(theme2Frags["header"], "theme2-header") {
		t.Fatalf("theme2 header missing theme2-header marker")
	}
	if !strings.Contains(theme2Frags["header"], "initTheme2Header") {
		t.Fatalf("theme2 header missing initTheme2Header function")
	}
	if !strings.Contains(theme2Frags["home_body"], "theme2-home") {
		t.Fatalf("theme2 home_body missing theme2-home marker")
	}
	if strings.Contains(theme2Frags["home_body"], "theme1-home") {
		t.Fatalf("theme2 home_body must not contain theme1-home")
	}
	if strings.Contains(theme2Frags["header"], "theme1-header") {
		t.Fatalf("theme2 header must not contain theme1-header")
	}
	if !strings.Contains(theme2Frags["sidebar"], "Pilihan Editor") {
		t.Fatalf("theme2 sidebar should use Pilihan Editor label")
	}
	if !strings.Contains(theme2Frags["sidebar"], `data-sidebar-source="latest"`) {
		t.Fatalf("theme2 sidebar should set data-sidebar-source=latest")
	}
	if !strings.Contains(theme2Frags["footer"], "theme2-footer") {
		t.Fatalf("theme2 footer missing theme2-footer marker")
	}
	if strings.Contains(theme2Frags["footer"], "bg-accent text-white") {
		t.Fatalf("theme2 footer should not use full accent bar like theme1")
	}
	if defaultFrags["home_body"] == theme2Frags["home_body"] {
		t.Fatalf("theme2 home_body should differ from default")
	}
	if theme1Frags["home_body"] == theme2Frags["home_body"] {
		t.Fatalf("theme2 home_body should differ from theme1")
	}
	if theme1Frags["header"] == theme2Frags["header"] {
		t.Fatalf("theme2 header should differ from theme1")
	}

	if !strings.Contains(theme3Frags["header"], "theme3-header") {
		t.Fatalf("theme3 header missing theme3-header marker")
	}
	if !strings.Contains(theme3Frags["header"], "initTheme3Header") {
		t.Fatalf("theme3 header missing initTheme3Header function")
	}
	if !strings.Contains(theme3Frags["home_body"], "theme3-home") {
		t.Fatalf("theme3 home_body missing theme3-home marker")
	}
	if !strings.Contains(theme3Frags["home_body"], "theme3-wire") {
		t.Fatalf("theme3 home_body missing theme3-wire desk layout")
	}
	if strings.Contains(theme3Frags["home_body"], "theme1-home") || strings.Contains(theme3Frags["home_body"], "theme2-home") {
		t.Fatalf("theme3 home_body must not contain theme1-home or theme2-home")
	}
	if strings.Contains(theme3Frags["header"], "theme1-header") || strings.Contains(theme3Frags["header"], "theme2-header") {
		t.Fatalf("theme3 header must not contain theme1/theme2 header markers")
	}
	if !strings.Contains(theme3Frags["sidebar"], "Terbaru") {
		t.Fatalf("theme3 sidebar should use Terbaru label")
	}
	if !strings.Contains(theme3Frags["sidebar"], `data-sidebar-source="latest"`) {
		t.Fatalf("theme3 sidebar should set data-sidebar-source=latest")
	}
	if !strings.Contains(theme3Frags["footer"], "theme3-footer") {
		t.Fatalf("theme3 footer missing theme3-footer marker")
	}
	if strings.Contains(theme3Frags["footer"], "bg-accent text-white") {
		t.Fatalf("theme3 footer should not use full accent bar like theme1")
	}
	if !strings.Contains(theme3Frags["article_body"], "theme3-share") {
		t.Fatalf("theme3 article_body missing theme3-share")
	}
	if theme2Frags["home_body"] == theme3Frags["home_body"] {
		t.Fatalf("theme3 home_body should differ from theme2")
	}
	if theme1Frags["home_body"] == theme3Frags["home_body"] {
		t.Fatalf("theme3 home_body should differ from theme1")
	}
}
