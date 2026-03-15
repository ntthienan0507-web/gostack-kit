//go:build integration

package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/ntthienan0507-web/gostack-kit/db/sqlc"
	"github.com/ntthienan0507-web/gostack-kit/modules/user"
	"github.com/ntthienan0507-web/gostack-kit/pkg/testutil"
)

func newIntegrationRepo(t *testing.T) user.Repository {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	queries := db.New(pool)
	return user.NewSQLCRepository(queries)
}

func TestIntegration_UserRepository_Create_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, user.CreateInput{
		Username:     "johndoe",
		Email:        "john@example.com",
		FullName:     "John Doe",
		PasswordHash: "hashed_password_123",
		Role:         "user",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "johndoe", created.Username)
	assert.Equal(t, "john@example.com", created.Email)
	assert.Equal(t, "John Doe", created.FullName)
	assert.Equal(t, "hashed_password_123", created.PasswordHash)
	assert.Equal(t, "user", created.Role)
	assert.True(t, created.IsActive)
	assert.False(t, created.CreatedAt.IsZero())
	assert.Nil(t, created.DeletedAt)
}

func TestIntegration_UserRepository_GetByID_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, user.CreateInput{
		Username: "jane",
		Email:    "jane@example.com",
		FullName: "Jane Doe",
		Role:     "admin",
	})
	require.NoError(t, err)

	found, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "jane", found.Username)
	assert.Equal(t, "jane@example.com", found.Email)
	assert.Equal(t, "admin", found.Role)
}

func TestIntegration_UserRepository_GetByEmail_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, user.CreateInput{
		Username: "alice",
		Email:    "alice@example.com",
		FullName: "Alice Smith",
		Role:     "user",
	})
	require.NoError(t, err)

	found, err := repo.GetByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, "alice", found.Username)
	assert.Equal(t, "alice@example.com", found.Email)
}

func TestIntegration_UserRepository_GetByUsername_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, user.CreateInput{
		Username: "bob",
		Email:    "bob@example.com",
		FullName: "Bob Jones",
		Role:     "user",
	})
	require.NoError(t, err)

	found, err := repo.GetByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.Equal(t, "bob", found.Username)
	assert.Equal(t, "bob@example.com", found.Email)
}

func TestIntegration_UserRepository_List_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	for _, input := range []user.CreateInput{
		{Username: "user1", Email: "user1@example.com", FullName: "User One", Role: "user"},
		{Username: "user2", Email: "user2@example.com", FullName: "User Two", Role: "admin"},
		{Username: "user3", Email: "user3@example.com", FullName: "User Three", Role: "user"},
	} {
		_, err := repo.Create(ctx, input)
		require.NoError(t, err)
	}

	// List all
	users, err := repo.List(ctx, user.ListParams{}, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 3)

	// List with role filter
	users, err = repo.List(ctx, user.ListParams{Role: "admin"}, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "admin", users[0].Role)

	// List with search
	users, err = repo.List(ctx, user.ListParams{Search: "Two"}, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "user2", users[0].Username)

	// List with pagination
	users, err = repo.List(ctx, user.ListParams{}, 2, 0)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestIntegration_UserRepository_Count_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	for _, input := range []user.CreateInput{
		{Username: "c1", Email: "c1@example.com", FullName: "Count One", Role: "user"},
		{Username: "c2", Email: "c2@example.com", FullName: "Count Two", Role: "admin"},
	} {
		_, err := repo.Create(ctx, input)
		require.NoError(t, err)
	}

	count, err := repo.Count(ctx, user.ListParams{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = repo.Count(ctx, user.ListParams{Role: "admin"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestIntegration_UserRepository_Update_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, user.CreateInput{
		Username: "updateme",
		Email:    "update@example.com",
		FullName: "Before Update",
		Role:     "user",
	})
	require.NoError(t, err)

	updated, err := repo.Update(ctx, user.UpdateInput{
		ID:       created.ID,
		FullName: "After Update",
		Role:     "admin",
		IsActive: true,
	})
	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "After Update", updated.FullName)
	assert.Equal(t, "admin", updated.Role)
	assert.True(t, updated.IsActive)
	assert.True(t, updated.UpdatedAt.After(created.UpdatedAt) || updated.UpdatedAt.Equal(created.UpdatedAt))
}

func TestIntegration_UserRepository_SoftDelete_Success(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, user.CreateInput{
		Username: "deleteme",
		Email:    "delete@example.com",
		FullName: "Delete Me",
		Role:     "user",
	})
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	// GetByID should fail for soft-deleted user (query filters deleted_at IS NULL)
	_, err = repo.GetByID(ctx, created.ID)
	assert.Error(t, err)

	// Count should not include soft-deleted user
	count, err := repo.Count(ctx, user.ListParams{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestIntegration_UserRepository_Create_DuplicateEmail(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, user.CreateInput{
		Username: "first",
		Email:    "dup@example.com",
		FullName: "First User",
		Role:     "user",
	})
	require.NoError(t, err)

	_, err = repo.Create(ctx, user.CreateInput{
		Username: "second",
		Email:    "dup@example.com",
		FullName: "Second User",
		Role:     "user",
	})
	assert.Error(t, err)
}

func TestIntegration_UserRepository_Create_DuplicateUsername(t *testing.T) {
	repo := newIntegrationRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, user.CreateInput{
		Username: "samename",
		Email:    "first@example.com",
		FullName: "First User",
		Role:     "user",
	})
	require.NoError(t, err)

	_, err = repo.Create(ctx, user.CreateInput{
		Username: "samename",
		Email:    "second@example.com",
		FullName: "Second User",
		Role:     "user",
	})
	assert.Error(t, err)
}
