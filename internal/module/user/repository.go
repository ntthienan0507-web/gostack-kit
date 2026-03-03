package user

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines data access for the user module.
// Uses domain types only — no SQLC or GORM types leak through.
// Both sqlcRepository and gormRepository implement this interface.
type Repository interface {
	List(ctx context.Context, params ListParams, limit, offset int32) ([]*User, error)
	Count(ctx context.Context, params ListParams) (int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, input CreateInput) (*User, error)
	Update(ctx context.Context, input UpdateInput) (*User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
