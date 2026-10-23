package dto

// CreateUserRequest is payload for creating admin users.
// Password is required.
type CreateUserRequest struct {
	Name     string `json:"name" binding:"required,min=3"`
	Username string `json:"username,omitempty" binding:"omitempty,min=3,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role,omitempty" binding:"omitempty,oneof=admin writer"`
	Enabled  *bool  `json:"enabled,omitempty"`
	Profile  string `json:"profile,omitempty"`
	Image    string `json:"image,omitempty"`
}

// UpdateUserRequest is payload for updating existing users.
// Password is optional; when omitted or empty, password won't be changed.
type UpdateUserRequest struct {
	Name     string  `json:"name" binding:"required,min=3"`
	Username *string `json:"username,omitempty" binding:"omitempty,min=3,max=100"`
	Email    string  `json:"email" binding:"required,email"`
	Password *string `json:"password,omitempty" binding:"omitempty,min=6"`
	Role     string  `json:"role,omitempty" binding:"omitempty,oneof=admin writer"`
	Enabled  *bool   `json:"enabled,omitempty"`
	Profile  *string `json:"profile,omitempty"`
	Image    *string `json:"image,omitempty"`
}
