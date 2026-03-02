package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	db "github.com/chungnguyen/go-api-template/db/sqlc"
	"github.com/chungnguyen/go-api-template/internal/response"
)

// Service holds business logic. No HTTP concerns.
type Service struct {
	repo   Repository
	logger *zap.Logger
}

// NewService creates a user service with injected repository.
func NewService(repo Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// List returns a paginated list of users.
func (s *Service) List(ctx context.Context, params ListParams) (*response.PaginatedResult, error) {
	limit := int32(params.PageSize)
	offset := int32((params.Page - 1) * params.PageSize)

	users, err := s.repo.List(ctx, db.ListUsersParams{
		Search:     params.Search,
		Role:       params.Role,
		PageSize:   limit,
		PageOffset: offset,
	})
	if err != nil {
		s.logger.Error("list users failed", zap.Error(err))
		return nil, err
	}

	count, err := s.repo.Count(ctx, db.CountUsersParams{
		Search: params.Search,
		Role:   params.Role,
	})
	if err != nil {
		s.logger.Error("count users failed", zap.Error(err))
		return nil, err
	}

	items := make([]UserResponse, len(users))
	for i, u := range users {
		items[i] = toResponse(u)
	}

	pagination := response.NewPagination(count, response.PaginationParams{
		Page:     params.Page,
		PageSize: params.PageSize,
	})

	return &response.PaginatedResult{Items: items, Pagination: pagination}, nil
}

// GetByID returns a single user by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*UserResponse, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	r := toResponse(u)
	return &r, nil
}

// Create creates a new user with hashed password.
func (s *Service) Create(ctx context.Context, params CreateParams) (*UserResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	role := params.Role
	if role == "" {
		role = "user"
	}

	u, err := s.repo.Create(ctx, db.CreateUserParams{
		Username:     params.Username,
		Email:        params.Email,
		FullName:     params.FullName,
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
		Role:         role,
	})
	if err != nil {
		return nil, err
	}

	r := toResponse(u)
	return &r, nil
}

// Update updates an existing user.
func (s *Service) Update(ctx context.Context, id uuid.UUID, params UpdateParams) (*UserResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updateParams := db.UpdateUserParams{
		ID:       id,
		FullName: existing.FullName,
		Role:     existing.Role,
		IsActive: existing.IsActive,
	}

	if params.FullName != nil {
		updateParams.FullName = *params.FullName
	}
	if params.Role != nil {
		updateParams.Role = *params.Role
	}
	if params.IsActive != nil {
		updateParams.IsActive = *params.IsActive
	}

	u, err := s.repo.Update(ctx, updateParams)
	if err != nil {
		return nil, err
	}

	r := toResponse(u)
	return &r, nil
}

// Delete soft-deletes a user.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}

func toResponse(u *db.User) UserResponse {
	resp := UserResponse{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		FullName: u.FullName,
		Role:     u.Role,
		IsActive: u.IsActive,
	}
	if u.CreatedAt.Valid {
		resp.CreatedAt = u.CreatedAt.Time
	}
	if u.UpdatedAt.Valid {
		resp.UpdatedAt = u.UpdatedAt.Time
	}
	return resp
}

