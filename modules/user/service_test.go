package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestService() (*Service, *MockRepository) {
	repo := new(MockRepository)
	logger := zap.NewNop()
	svc := NewService(repo, logger)
	return svc, repo
}

func sampleUser() *User {
	return &User{
		ID:           uuid.New(),
		Username:     "johndoe",
		Email:        "john@example.com",
		FullName:     "John Doe",
		PasswordHash: "$2a$10$hashedpassword",
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// --- List ---

func TestService_List_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 1, PageSize: 20}

	users := []*User{sampleUser(), sampleUser()}
	repo.On("List", ctx, params, int32(20), int32(0)).Return(users, nil)
	repo.On("Count", ctx, params).Return(int64(2), nil)

	result, err := svc.List(ctx, params)

	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, int64(2), result.Total)
	repo.AssertExpectations(t)
}

func TestService_List_RepoListError(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 1, PageSize: 20}

	repo.On("List", ctx, params, int32(20), int32(0)).Return(nil, errors.New("db error"))

	result, err := svc.List(ctx, params)

	assert.Nil(t, result)
	assert.EqualError(t, err, "db error")
}

func TestService_List_RepoCountError(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 1, PageSize: 20}

	repo.On("List", ctx, params, int32(20), int32(0)).Return([]*User{}, nil)
	repo.On("Count", ctx, params).Return(int64(0), errors.New("count error"))

	result, err := svc.List(ctx, params)

	assert.Nil(t, result)
	assert.EqualError(t, err, "count error")
}

func TestService_List_Pagination(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	params := ListParams{Page: 3, PageSize: 10}

	repo.On("List", ctx, params, int32(10), int32(20)).Return([]*User{}, nil)
	repo.On("Count", ctx, params).Return(int64(25), nil)

	result, err := svc.List(ctx, params)

	require.NoError(t, err)
	assert.Empty(t, result.Items)
	assert.Equal(t, int64(25), result.Total)
}

// --- GetByID ---

func TestService_GetByID_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	u := sampleUser()

	repo.On("GetByID", ctx, u.ID).Return(u, nil)

	result, err := svc.GetByID(ctx, u.ID)

	require.NoError(t, err)
	assert.Equal(t, u.ID, result.ID)
	assert.Equal(t, u.Email, result.Email)
	assert.Equal(t, u.Username, result.Username)
}

func TestService_GetByID_NotFound(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, pgx.ErrNoRows)

	result, err := svc.GetByID(ctx, id)

	assert.Nil(t, result)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// --- Create ---

func TestService_Create_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	req := CreateRequest{
		Username: "newuser",
		Email:    "new@example.com",
		FullName: "New User",
		Password: "securepassword",
		Role:     "admin",
	}

	repo.On("Create", ctx, mock.MatchedBy(func(input CreateInput) bool {
		return input.Username == "newuser" &&
			input.Email == "new@example.com" &&
			input.FullName == "New User" &&
			input.Role == "admin" &&
			input.PasswordHash != "" &&
			input.PasswordHash != "securepassword" // must be hashed
	})).Return(sampleUser(), nil)

	result, err := svc.Create(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	repo.AssertExpectations(t)
}

func TestService_Create_DefaultRole(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	req := CreateRequest{
		Username: "newuser",
		Email:    "new@example.com",
		FullName: "New User",
		Password: "securepassword",
	}

	repo.On("Create", ctx, mock.MatchedBy(func(input CreateInput) bool {
		return input.Role == "user"
	})).Return(sampleUser(), nil)

	_, err := svc.Create(ctx, req)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_Create_RepoError(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()

	req := CreateRequest{
		Username: "newuser",
		Email:    "new@example.com",
		FullName: "New User",
		Password: "securepassword",
	}

	repo.On("Create", ctx, mock.Anything).Return(nil, errors.New("duplicate"))

	result, err := svc.Create(ctx, req)

	assert.Nil(t, result)
	assert.EqualError(t, err, "duplicate")
}

// --- Update ---

func TestService_Update_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	u := sampleUser()
	newName := "Updated Name"
	newRole := "admin"

	repo.On("GetByID", ctx, u.ID).Return(u, nil)
	repo.On("Update", ctx, UpdateInput{
		ID:       u.ID,
		FullName: newName,
		Role:     newRole,
		IsActive: u.IsActive,
	}).Return(&User{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		FullName: newName,
		Role:     newRole,
		IsActive: u.IsActive,
	}, nil)

	result, err := svc.Update(ctx, u.ID, UpdateRequest{
		FullName: &newName,
		Role:     &newRole,
	})

	require.NoError(t, err)
	assert.Equal(t, newName, result.FullName)
	assert.Equal(t, newRole, result.Role)
}

func TestService_Update_PartialUpdate(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	u := sampleUser()
	newName := "Only Name Changed"

	repo.On("GetByID", ctx, u.ID).Return(u, nil)
	repo.On("Update", ctx, UpdateInput{
		ID:       u.ID,
		FullName: newName,
		Role:     u.Role,
		IsActive: u.IsActive,
	}).Return(&User{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		FullName: newName,
		Role:     u.Role,
		IsActive: u.IsActive,
	}, nil)

	result, err := svc.Update(ctx, u.ID, UpdateRequest{
		FullName: &newName,
	})

	require.NoError(t, err)
	assert.Equal(t, newName, result.FullName)
	assert.Equal(t, u.Role, result.Role)
}

func TestService_Update_NotFound(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, pgx.ErrNoRows)

	result, err := svc.Update(ctx, id, UpdateRequest{})

	assert.Nil(t, result)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// --- Delete ---

func TestService_Delete_Success(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	u := sampleUser()

	repo.On("GetByID", ctx, u.ID).Return(u, nil)
	repo.On("SoftDelete", ctx, u.ID).Return(nil)

	err := svc.Delete(ctx, u.ID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestService_Delete_NotFound(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	id := uuid.New()

	repo.On("GetByID", ctx, id).Return(nil, pgx.ErrNoRows)

	err := svc.Delete(ctx, id)

	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestService_Delete_SoftDeleteError(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	u := sampleUser()

	repo.On("GetByID", ctx, u.ID).Return(u, nil)
	repo.On("SoftDelete", ctx, u.ID).Return(errors.New("db error"))

	err := svc.Delete(ctx, u.ID)

	assert.EqualError(t, err, "db error")
}

// --- ToResponse ---

func TestToResponse_NoPasswordHash(t *testing.T) {
	u := sampleUser()
	u.PasswordHash = "super-secret-hash"

	resp := ToResponse(u)

	assert.Equal(t, u.ID, resp.ID)
	assert.Equal(t, u.Username, resp.Username)
	assert.Equal(t, u.Email, resp.Email)
	assert.Equal(t, u.FullName, resp.FullName)
	assert.Equal(t, u.Role, resp.Role)
	assert.Equal(t, u.IsActive, resp.IsActive)
	assert.Equal(t, u.CreatedAt, resp.CreatedAt)
	assert.Equal(t, u.UpdatedAt, resp.UpdatedAt)
}
