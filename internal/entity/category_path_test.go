package entity

import "testing"

func TestCategoryPublicWebPath(t *testing.T) {
	root := Category{Slug: "tech"}
	if got := CategoryPublicWebPath(root); got != "/tech" {
		t.Fatalf("root: got %q", got)
	}
	pid := uint(1)
	parent := Category{ID: 1, Slug: "tech"}
	sub := Category{Slug: "mobile", ParentID: &pid, Parent: &parent}
	if got := CategoryPublicWebPath(sub); got != "/tech/mobile" {
		t.Fatalf("sub: got %q", got)
	}
	if got := CategoryWebPathPrefix(sub); got != "tech/mobile" {
		t.Fatalf("prefix: got %q", got)
	}
}

func TestPublishedArticleWebPathNestedCategory(t *testing.T) {
	got := PublishedArticleWebPath(ArticleSlugModeCategory, "tech/mobile", "my-post")
	if got != "/tech/mobile/my-post" {
		t.Fatalf("got %q", got)
	}
}
