//go:build integration

package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ntthienan0507-web/gostack-kit/modules/user"
	"github.com/ntthienan0507-web/gostack-kit/pkg/testutil"
)

func newGormRepo(t *testing.T) user.Repository {
	t.Helper()
	db := testutil.NewGormDB(t)
	return user.NewGORMRepository(db)
}

func TestIntegration_GORMRepository_Create_Success(t *testing.T) {
	repo := newGormRepo(t)
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
}

func TestIntegration_GORMRepository_GetByID_Success(t *testing.T) {
	repo := newGormRepo(t)
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
	assert.Equal(t, "admin", found.Role)
}

func TestIntegration_GORMRepository_GetByEmail_Success(t *testing.T) {
	repo := newGormRepo(t)
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
}

func TestIntegration_GORMRepository_GetByUsername_Success(t *testing.T) {
	repo := newGormRepo(t)
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
	assert.Equal(t, "bob@example.com", found.Email)
}

func TestIntegration_GORMRepository_List_Success(t *testing.T) {
	repo := newGormRepo(t)
	ctx := context.Background()

	for _, input := range []user.CreateInput{
		{Username: "g1", Email: "g1@example.com", FullName: "User One", Role: "user"},
		{Username: "g2", Email: "g2@example.com", FullName: "User Two", Role: "admin"},
		{Username: "g3", Email: "g3@example.com", FullName: "User Three", Role: "user"},
	} {
		_, err := repo.Create(ctx, input)
		require.NoError(t, err)
	}

	users, err := repo.List(ctx, user.ListParams{}, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 3)

	users, err = repo.List(ctx, user.ListParams{Role: "admin"}, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "admin", users[0].Role)

	users, err = repo.List(ctx, user.ListParams{Search: "Two"}, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "g2", users[0].Username)

	users, err = repo.List(ctx, user.ListParams{}, 2, 0)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestIntegration_GORMRepository_Count_Success(t *testing.T) {
	repo := newGormRepo(t)
	ctx := context.Background()

	for _, input := range []user.CreateInput{
		{Username: "gc1", Email: "gc1@example.com", FullName: "Count One", Role: "user"},
		{Username: "gc2", Email: "gc2@example.com", FullName: "Count Two", Role: "admin"},
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

func TestIntegration_GORMRepository_Update_Success(t *testing.T) {
	repo := newGormRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, user.CreateInput{
		Username: "gupdateme",
		Email:    "gupdate@example.com",
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
}

func TestIntegration_GORMRepository_SoftDelete_Success(t *testing.T) {
	repo := newGormRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, user.CreateInput{
		Username: "gdeleteme",
		Email:    "gdelete@example.com",
		FullName: "Delete Me",
		Role:     "user",
	})
	require.NoError(t, err)

	err = repo.SoftDelete(ctx, created.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, created.ID)
	assert.Error(t, err)

	count, err := repo.Count(ctx, user.ListParams{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestIntegration_GORMRepository_Create_DuplicateEmail(t *testing.T) {
	repo := newGormRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, user.CreateInput{
		Username: "gfirst",
		Email:    "gdup@example.com",
		FullName: "First",
		Role:     "user",
	})
	require.NoError(t, err)

	_, err = repo.Create(ctx, user.CreateInput{
		Username: "gsecond",
		Email:    "gdup@example.com",
		FullName: "Second",
		Role:     "user",
	})
	assert.Error(t, err)
}

func TestIntegration_GORMRepository_Create_DuplicateUsername(t *testing.T) {
	repo := newGormRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, user.CreateInput{
		Username: "gsamename",
		Email:    "gfirst@example.com",
		FullName: "First",
		Role:     "user",
	})
	require.NoError(t, err)

	_, err = repo.Create(ctx, user.CreateInput{
		Username: "gsamename",
		Email:    "gsecond@example.com",
		FullName: "Second",
		Role:     "user",
	})
	assert.Error(t, err)
}
