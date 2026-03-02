package user

import (
	"context"

	"github.com/google/uuid"

	db "github.com/chungnguyen/go-api-template/db/sqlc"
)

// Repository defines data access for the user module.
// Service depends on this interface — testable via mock.
type Repository interface {
	List(ctx context.Context, params db.ListUsersParams) ([]*db.User, error)
	Count(ctx context.Context, params db.CountUsersParams) (int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*db.User, error)
	GetByEmail(ctx context.Context, email string) (*db.User, error)
	GetByUsername(ctx context.Context, username string) (*db.User, error)
	Create(ctx context.Context, params db.CreateUserParams) (*db.User, error)
	Update(ctx context.Context, params db.UpdateUserParams) (*db.User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// sqlcRepository implements Repository using SQLC-generated code.
type sqlcRepository struct {
	q *db.Queries
}

// NewRepository creates a SQLC-backed user repository.
func NewRepository(q *db.Queries) Repository {
	return &sqlcRepository{q: q}
}

func (r *sqlcRepository) List(ctx context.Context, params db.ListUsersParams) ([]*db.User, error) {
	return r.q.ListUsers(ctx, params)
}

func (r *sqlcRepository) Count(ctx context.Context, params db.CountUsersParams) (int64, error) {
	return r.q.CountUsers(ctx, params)
}

func (r *sqlcRepository) GetByID(ctx context.Context, id uuid.UUID) (*db.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *sqlcRepository) GetByEmail(ctx context.Context, email string) (*db.User, error) {
	return r.q.GetUserByEmail(ctx, email)
}

func (r *sqlcRepository) GetByUsername(ctx context.Context, username string) (*db.User, error) {
	return r.q.GetUserByUsername(ctx, username)
}

func (r *sqlcRepository) Create(ctx context.Context, params db.CreateUserParams) (*db.User, error) {
	return r.q.CreateUser(ctx, params)
}

func (r *sqlcRepository) Update(ctx context.Context, params db.UpdateUserParams) (*db.User, error) {
	return r.q.UpdateUser(ctx, params)
}

func (r *sqlcRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.q.SoftDeleteUser(ctx, id)
}
