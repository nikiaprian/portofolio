package services

import "errors"

var (
	ErrCategoryParentNotFound = errors.New("parent category not found")
	ErrCategoryInvalidParent  = errors.New("invalid parent category")
	ErrCategoryMaxDepth       = errors.New("subcategories cannot have their own subcategories")
	ErrCategoryHasChildren    = errors.New("category has subcategories")
)
