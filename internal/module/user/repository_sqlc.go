package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/ntthienan0507-web/go-api-template/db/sqlc"
)

// sqlcRepository implements Repository using SQLC-generated code.
type sqlcRepository struct {
	q *db.Queries
}

// NewSQLCRepository creates a SQLC-backed user repository.
func NewSQLCRepository(q *db.Queries) Repository {
	return &sqlcRepository{q: q}
}

func (r *sqlcRepository) List(ctx context.Context, params ListParams, limit, offset int32) ([]*User, error) {
	rows, err := r.q.ListUsers(ctx, db.ListUsersParams{
		Search:     params.Search,
		Role:       params.Role,
		PageSize:   limit,
		PageOffset: offset,
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, fromDBUser), nil
}

func (r *sqlcRepository) Count(ctx context.Context, params ListParams) (int64, error) {
	return r.q.CountUsers(ctx, db.CountUsersParams{
		Search: params.Search,
		Role:   params.Role,
	})
}

func (r *sqlcRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u := fromDBUser(row)
	return u, nil
}

func (r *sqlcRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	u := fromDBUser(row)
	return u, nil
}

func (r *sqlcRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	row, err := r.q.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	u := fromDBUser(row)
	return u, nil
}

func (r *sqlcRepository) Create(ctx context.Context, input CreateInput) (*User, error) {
	row, err := r.q.CreateUser(ctx, db.CreateUserParams{
		Username:     input.Username,
		Email:        input.Email,
		FullName:     input.FullName,
		PasswordHash: pgtype.Text{String: input.PasswordHash, Valid: input.PasswordHash != ""},
		Role:         input.Role,
	})
	if err != nil {
		return nil, err
	}
	u := fromDBUser(row)
	return u, nil
}

func (r *sqlcRepository) Update(ctx context.Context, input UpdateInput) (*User, error) {
	row, err := r.q.UpdateUser(ctx, db.UpdateUserParams{
		ID:       input.ID,
		FullName: input.FullName,
		Role:     input.Role,
		IsActive: input.IsActive,
	})
	if err != nil {
		return nil, err
	}
	u := fromDBUser(row)
	return u, nil
}

func (r *sqlcRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.q.SoftDeleteUser(ctx, id)
}

// --- Mapping helpers: SQLC db.User ↔ domain User ---

func fromDBUser(row *db.User) *User {
	u := &User{
		ID:       row.ID,
		Username: row.Username,
		Email:    row.Email,
		FullName: row.FullName,
		Role:     row.Role,
		IsActive: row.IsActive,
		Metadata: row.Metadata,
	}
	if row.PasswordHash.Valid {
		u.PasswordHash = row.PasswordHash.String
	}
	if row.CreatedAt.Valid {
		u.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		u.UpdatedAt = row.UpdatedAt.Time
	}
	if row.DeletedAt.Valid {
		u.DeletedAt = &row.DeletedAt.Time
	}
	return u
}

func mapSlice[T any, U any](items []T, fn func(T) U) []U {
	result := make([]U, len(items))
	for i, item := range items {
		result[i] = fn(item)
	}
	return result
}
