package user

import (
	"time"

	"github.com/google/uuid"
)

// ListParams are validated inputs for the List operation.
type ListParams struct {
	Search   string
	Role     string
	Page     int
	PageSize int
}

// CreateParams are validated inputs for the Create operation.
type CreateParams struct {
	Username string `json:"username" binding:"required,min=3,max=100"`
	Email    string `json:"email"    binding:"required,email"`
	FullName string `json:"full_name" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role"     binding:"omitempty,oneof=user admin"`
}

// UpdateParams are validated inputs for the Update operation.
type UpdateParams struct {
	FullName *string `json:"full_name"`
	Role     *string `json:"role" binding:"omitempty,oneof=user admin"`
	IsActive *bool   `json:"is_active"`
}

// UserResponse is the client-facing representation. Never exposes password_hash.
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
