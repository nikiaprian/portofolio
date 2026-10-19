package dto

// CreateTagRequest validation
type CreateTagRequest struct {
	Name string `json:"name" binding:"required,min=3,max=100"`
	Slug string `json:"slug" binding:"required,min=3,max=150,slug"`
}

// UpdateTagRequest validation
type UpdateTagRequest struct {
	Name *string `json:"name" binding:"required,min=3,max=100"`
	Slug *string `json:"slug" binding:"required,min=3,max=150,slug"`
}
