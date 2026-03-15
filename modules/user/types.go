package user

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Domain Model (ORM-agnostic) ---

// User is the domain model used across service and repository layers.
// Neither SQLC nor GORM types leak outside the repository.
type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	FullName     string
	PasswordHash string
	Role         string
	IsActive     bool
	Metadata     json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// --- Query Params (domain-level, ORM-agnostic) ---

// ListParams are validated inputs for the List operation.
type ListParams struct {
	Search   string
	Role     string
	Page     int
	PageSize int
}

// CreateInput are validated inputs for the Create operation.
type CreateInput struct {
	Username     string
	Email        string
	FullName     string
	PasswordHash string
	Role         string
}

// UpdateInput are validated inputs for the Update operation.
type UpdateInput struct {
	ID       uuid.UUID
	FullName string
	Role     string
	IsActive bool
}

// --- HTTP Request/Response types ---

// CreateRequest is the JSON body for POST /users.
type CreateRequest struct {
	Username string `json:"username" binding:"required,min=3,max=100"`
	Email    string `json:"email"    binding:"required,email"`
	FullName string `json:"full_name" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role"     binding:"omitempty,oneof=user admin"`
}

// UpdateRequest is the JSON body for PUT /users/:id.
type UpdateRequest struct {
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

// ToResponse converts domain User to client-facing UserResponse.
func ToResponse(u *User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      u.Role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
