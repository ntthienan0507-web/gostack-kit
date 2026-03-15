package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/ntthienan0507-web/go-api-template/pkg/response"
)

// Service holds business logic. No HTTP concerns, no ORM dependency.
// Works with any Repository implementation (SQLC or GORM).
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

	users, err := s.repo.List(ctx, params, limit, offset)
	if err != nil {
		s.logger.Error("list users failed", zap.Error(err))
		return nil, err
	}

	count, err := s.repo.Count(ctx, params)
	if err != nil {
		s.logger.Error("count users failed", zap.Error(err))
		return nil, err
	}

	items := make([]UserResponse, len(users))
	for i, u := range users {
		items[i] = ToResponse(u)
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
	r := ToResponse(u)
	return &r, nil
}

// Create creates a new user with hashed password.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*UserResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	u, err := s.repo.Create(ctx, CreateInput{
		Username:     req.Username,
		Email:        req.Email,
		FullName:     req.FullName,
		PasswordHash: string(hash),
		Role:         role,
	})
	if err != nil {
		return nil, err
	}

	r := ToResponse(u)
	return &r, nil
}

// Update updates an existing user.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*UserResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	input := UpdateInput{
		ID:       id,
		FullName: existing.FullName,
		Role:     existing.Role,
		IsActive: existing.IsActive,
	}

	if req.FullName != nil {
		input.FullName = *req.FullName
	}
	if req.Role != nil {
		input.Role = *req.Role
	}
	if req.IsActive != nil {
		input.IsActive = *req.IsActive
	}

	u, err := s.repo.Update(ctx, input)
	if err != nil {
		return nil, err
	}

	r := ToResponse(u)
	return &r, nil
}

// Delete soft-deletes a user.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}
