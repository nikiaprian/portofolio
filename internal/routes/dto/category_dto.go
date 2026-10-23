package dto

// CreateCategoryRequest validation
type CreateCategoryRequest struct {
	Sort        *int   `json:"sort" binding:"omitempty,min=1"`
	ParentID    *uint  `json:"parent_id" binding:"omitempty,gt=0"`
	Name        string `json:"name" binding:"required,min=3,max=100"`
	Slug        string `json:"slug" binding:"required,min=3,max=150,slug"`
	Title       string `json:"title" binding:"max=255"`
	Keyword     string `json:"keyword" binding:"max=500"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateCategoryRequest validation
type UpdateCategoryRequest struct {
	Sort        *int    `json:"sort" binding:"omitempty,min=1"`
	ParentID    *uint   `json:"parent_id" binding:"omitempty,gt=0"`
	ClearParent *bool   `json:"clear_parent"`
	Name        *string `json:"name" binding:"required,min=3,max=100"`
	Slug        *string `json:"slug" binding:"required,min=3,max=150,slug"`
	Title       *string `json:"title" binding:"omitempty,max=255"`
	Keyword     *string `json:"keyword" binding:"omitempty,max=500"`
	Description *string `json:"description" binding:"max=500"`
}
