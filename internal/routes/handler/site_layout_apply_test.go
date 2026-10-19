package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyLayoutFragmentsRoundTrip(t *testing.T) {
	root := findRepoRoot(t)
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	backup := map[string][]byte{}
	for frag, rel := range siteLayoutFragmentRefFiles {
		b, readErr := os.ReadFile(rel)
		if readErr != nil {
			t.Fatalf("backup %s: %v", frag, readErr)
		}
		backup[frag] = b
	}
	t.Cleanup(func() {
		for frag, content := range backup {
			_ = os.WriteFile(siteLayoutFragmentRefFiles[frag], content, 0o644)
		}
	})

	theme1Dir := filepath.Join(root, "db", "seeds", "layouts", "theme1")
	theme1Files := map[string]string{
		"header":         "header.html",
		"footer":         "footer.html",
		"home_body":      "home_body.html",
		"article_body":   "article_body.html",
		"sidebar":        "sidebar.html",
		"rss":            "rss.html",
		"author_profile": "author_profile.html",
		"tag":            "tag.html",
		"kategori":       "kategori.html",
		"search_page":    "search_page.html",
	}

	for _, frag := range siteLayoutFragmentKeys {
		content, readErr := os.ReadFile(filepath.Join(theme1Dir, theme1Files[frag]))
		if readErr != nil {
			t.Fatalf("read theme1 %s: %v", frag, readErr)
		}
		if _, applyErr := applyFragmentToTemplateFile(frag, string(content)); applyErr != nil {
			t.Fatalf("apply theme1 %s: %v", frag, applyErr)
		}
	}

	header, err := os.ReadFile(siteLayoutFragmentRefFiles["header"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(header), "theme1-header") {
		t.Fatalf("header after theme1 apply missing theme1-header marker")
	}

	home, err := os.ReadFile(siteLayoutFragmentRefFiles["home_body"])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(home), "Artikel Kegiatan") {
		t.Fatalf("home after theme1 apply missing Artikel Kegiatan section")
	}
	if strings.Contains(string(home), "Video Kegiatan") {
		t.Fatalf("home after theme1 apply still has Video Kegiatan")
	}

	// Restore default (live backup) via apply
	for _, frag := range siteLayoutFragmentKeys {
		if _, applyErr := applyFragmentToTemplateFile(frag, string(backup[frag])); applyErr != nil {
			t.Fatalf("restore %s: %v", frag, applyErr)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, e1 := os.Stat(filepath.Join(dir, "go.mod")); e1 == nil {
			if _, e2 := os.Stat(filepath.Join(dir, "public", "templates")); e2 == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repo root not found from %s", wd)
	return ""
}
