package entity

import "testing"

func TestBuildNavCategoryTree(t *testing.T) {
	pid := uint(1)
	cats := []Category{
		{ID: 1, Name: "Tech", Slug: "tech", Sort: 1},
		{ID: 2, Name: "Mobile", Slug: "mobile", Sort: 1, ParentID: &pid},
		{ID: 3, Name: "Life", Slug: "life", Sort: 2},
	}
	tree := BuildNavCategoryTree(cats)
	if len(tree) != 2 {
		t.Fatalf("roots: got %d want 2", len(tree))
	}
	if tree[0].Slug != "tech" || len(tree[0].Children) != 1 {
		t.Fatalf("tech node: %+v", tree[0])
	}
	if tree[0].Children[0].Slug != "mobile" {
		t.Fatalf("child slug: %s", tree[0].Children[0].Slug)
	}
}

func TestSortCategoriesHierarchical(t *testing.T) {
	pid := uint(10)
	cats := []Category{
		{ID: 11, Name: "B", ParentID: &pid},
		{ID: 10, Name: "Root"},
		{ID: 12, Name: "A", ParentID: &pid},
	}
	sorted := SortCategoriesHierarchical(cats)
	if len(sorted) != 3 {
		t.Fatalf("len %d", len(sorted))
	}
	if sorted[0].ID != 10 || sorted[1].ID != 12 || sorted[2].ID != 11 {
		t.Fatalf("order: %+v", sorted)
	}
}

func TestCategoryIsRoot(t *testing.T) {
	zero := uint(0)
	c := Category{ParentID: &zero}
	if !c.IsRoot() {
		t.Fatal("zero parent should count as root")
	}
}
