package dto

import "github.com/nikiaprian/tirtayasa/internal/entity"

// PaginatedUsersResponse is the API response for GET /admin/users.
type PaginatedUsersResponse struct {
	Items      []entity.User `json:"items"`
	Page       int           `json:"page"`
	Size       int           `json:"size"`
	Total      int64         `json:"total"`
	TotalPages int           `json:"total_pages"`
}

// PaginatedPagesResponse is the API response for GET /admin/pages.
type PaginatedPagesResponse struct {
	Items      []entity.Page `json:"items"`
	Page       int           `json:"page"`
	Size       int           `json:"size"`
	Total      int64         `json:"total"`
	TotalPages int           `json:"total_pages"`
}

// PaginatedPostsResponse is the API response for GET /admin/posts.
type PaginatedPostsResponse struct {
	Items      []entity.Post `json:"items"`
	Page       int           `json:"page"`
	Size       int           `json:"size"`
	Total      int64         `json:"total"`
	TotalPages int           `json:"total_pages"`
}

// PaginatedCategoriesResponse is the API response for GET /admin/categories.
type PaginatedCategoriesResponse struct {
	Items      []entity.Category `json:"items"`
	Page       int               `json:"page"`
	Size       int               `json:"size"`
	Total      int64             `json:"total"`
	TotalPages int               `json:"total_pages"`
}

// PaginatedTagsResponse is the API response for GET /admin/tags.
type PaginatedTagsResponse struct {
	Items      []entity.Tag `json:"items"`
	Page       int          `json:"page"`
	Size       int          `json:"size"`
	Total      int64        `json:"total"`
	TotalPages int          `json:"total_pages"`
}
