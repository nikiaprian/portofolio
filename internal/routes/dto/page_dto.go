package dto

// CreatePageRequest validation
type CreatePageRequest struct {
	Title   string `json:"title" binding:"required,min=3,max=200"`
	Slug    string `json:"slug" binding:"required,min=3,max=255,slug"`
	Content string `json:"content" binding:"required,min=10,max=65535"`
	Status  string `json:"status" binding:"required,oneof=draft published"`
}

// UpdatePageRequest validation
type UpdatePageRequest struct {
	Title   *string `json:"title" binding:"required,min=3,max=200"`
	Slug    *string `json:"slug" binding:"required,min=3,max=255,slug"`
	Content *string `json:"content" binding:"required,min=10,max=65535"`
	Status  *string `json:"status" binding:"required,oneof=draft published"`
}
