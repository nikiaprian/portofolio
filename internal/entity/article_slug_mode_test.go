package entity

import "testing"

func TestPublishedArticleWebPath(t *testing.T) {
	tests := []struct {
		mode     string
		catSlug  string
		postSlug string
		want     string
	}{
		{ArticleSlugModeDirect, "teknologi", "judul-artikel", "/judul-artikel"},
		{ArticleSlugModeDirect, "", "judul-artikel", "/judul-artikel"},
		{ArticleSlugModeCategory, "teknologi", "judul-artikel", "/teknologi/judul-artikel"},
		{ArticleSlugModeCategory, "", "judul-artikel", "/judul-artikel"},
		{ArticleSlugModeCategoryNested, "nasional/anjay", "judul-artikel", "/nasional/anjay/judul-artikel"},
		{"", "teknologi", "judul-artikel", "/judul-artikel"},
	}
	for _, tc := range tests {
		got := PublishedArticleWebPath(tc.mode, tc.catSlug, tc.postSlug)
		if got != tc.want {
			t.Errorf("PublishedArticleWebPath(%q, %q, %q) = %q, want %q",
				tc.mode, tc.catSlug, tc.postSlug, got, tc.want)
		}
	}
}

func TestCategoryPathPrefixForArticleURL(t *testing.T) {
	root := Category{ID: 1, Slug: "nasional"}
	pid := uint(1)
	sub := Category{ID: 2, Slug: "anjay", ParentID: &pid, Parent: &root}

	if got := CategoryPathPrefixForArticleURL(ArticleSlugModeCategory, root); got != "nasional" {
		t.Fatalf("root category: got %q", got)
	}
	if got := CategoryPathPrefixForArticleURL(ArticleSlugModeCategory, sub); got != "anjay" {
		t.Fatalf("sub category flat mode: got %q, want anjay", got)
	}
	if got := CategoryPathPrefixForArticleURL(ArticleSlugModeCategoryNested, sub); got != "nasional/anjay" {
		t.Fatalf("sub category nested mode: got %q, want nasional/anjay", got)
	}
}

func TestIsValidArticleSlugMode(t *testing.T) {
	for _, m := range []string{"direct", "category", "category_nested", "CATEGORY_NESTED"} {
		if !IsValidArticleSlugMode(m) {
			t.Fatalf("expected valid: %q", m)
		}
	}
	if IsValidArticleSlugMode("category-nested") {
		t.Fatal("expected invalid")
	}
}

func TestPublishedCategoryWebPath(t *testing.T) {
	if got := PublishedCategoryWebPath("teknologi"); got != "/teknologi" {
		t.Errorf("got %q, want /teknologi", got)
	}
	if got := PublishedCategoryWebPath(""); got != "/" {
		t.Errorf("empty slug: got %q, want /", got)
	}
}
