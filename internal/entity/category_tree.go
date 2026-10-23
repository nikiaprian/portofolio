package entity

import "sort"

// NavCategoryNode is a root category with optional subcategories for site navigation (one level).
type NavCategoryNode struct {
	ID       uint              `json:"id"`
	Name     string            `json:"name"`
	Slug     string            `json:"slug"`
	Sort     int               `json:"sort"`
	Children []NavCategoryChild `json:"children,omitempty"`
}

// NavCategoryChild is a subcategory link in the public nav.
type NavCategoryChild struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	ParentSlug string `json:"parent_slug"`
	Sort       int    `json:"sort"`
}

// BuildNavCategoryTree groups flat categories into roots with children (sort preserved).
func BuildNavCategoryTree(cats []Category) []NavCategoryNode {
	if len(cats) == 0 {
		return nil
	}
	childrenOf := make(map[uint][]Category)
	var roots []Category
	for _, c := range cats {
		if c.IsRoot() {
			roots = append(roots, c)
			continue
		}
		pid := *c.ParentID
		childrenOf[pid] = append(childrenOf[pid], c)
	}
	sort.Slice(roots, func(i, j int) bool {
		return categoryNavLess(roots[i], roots[j])
	})
	out := make([]NavCategoryNode, 0, len(roots))
	for _, r := range roots {
		node := NavCategoryNode{
			ID:   r.ID,
			Name: r.Name,
			Slug: r.Slug,
			Sort: r.Sort,
		}
		kids := childrenOf[r.ID]
		sort.Slice(kids, func(i, j int) bool {
			return categoryNavLess(kids[i], kids[j])
		})
		for _, ch := range kids {
			node.Children = append(node.Children, NavCategoryChild{
				ID:         ch.ID,
				Name:       ch.Name,
				Slug:       ch.Slug,
				ParentSlug: r.Slug,
				Sort:       ch.Sort,
			})
		}
		out = append(out, node)
	}
	return out
}

func categoryNavLess(a, b Category) bool {
	sa, sb := a.Sort, b.Sort
	if sa < 1 {
		sa = 1 << 30
	}
	if sb < 1 {
		sb = 1 << 30
	}
	if sa != sb {
		return sa < sb
	}
	na, nb := a.Name, b.Name
	if na != nb {
		return na < nb
	}
	return a.ID < b.ID
}

// SortCategoriesHierarchical returns roots first, then each root's children (for admin selects).
func SortCategoriesHierarchical(cats []Category) []Category {
	if len(cats) == 0 {
		return nil
	}
	childrenOf := make(map[uint][]Category)
	var roots []Category
	for _, c := range cats {
		if c.IsRoot() {
			roots = append(roots, c)
			continue
		}
		pid := *c.ParentID
		childrenOf[pid] = append(childrenOf[pid], c)
	}
	sort.Slice(roots, func(i, j int) bool {
		return categoryNavLess(roots[i], roots[j])
	})
	out := make([]Category, 0, len(cats))
	for _, r := range roots {
		out = append(out, r)
		kids := childrenOf[r.ID]
		sort.Slice(kids, func(i, j int) bool {
			return categoryNavLess(kids[i], kids[j])
		})
		out = append(out, kids...)
	}
	return out
}

// CategoryOptionLabel returns a display label for admin dropdowns (indents subcategories).
func CategoryOptionLabel(c Category) string {
	if c.IsRoot() {
		return c.Name
	}
	return "— " + c.Name
}
