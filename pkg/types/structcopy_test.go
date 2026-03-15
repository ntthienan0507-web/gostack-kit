package types

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// Simulate SQLC generated structs
type listUsersParams struct {
	Search pgtype.Text
	Role   pgtype.Text
	Status pgtype.Text
	Limit  int32
	Offset int32
}

type countUsersParams struct {
	Search pgtype.Text
	Role   pgtype.Text
	Status pgtype.Text
}

func TestCopyFields_QueryToCount(t *testing.T) {
	query := listUsersParams{
		Search: pgtype.Text{String: "john", Valid: true},
		Role:   pgtype.Text{String: "admin", Valid: true},
		Status: pgtype.Text{String: "active", Valid: true},
		Limit:  20,
		Offset: 40,
	}

	count := CopyFields[countUsersParams](query)

	// Shared fields copied
	assert.Equal(t, query.Search, count.Search)
	assert.Equal(t, query.Role, count.Role)
	assert.Equal(t, query.Status, count.Status)
}

func TestCopyFields_SkipsLimitOffset(t *testing.T) {
	type withPagination struct {
		Name   string
		Limit  int32
		Offset int32
	}
	type withoutPagination struct {
		Name   string
		Limit  int32 // even if target HAS Limit, it should be skipped
		Offset int32
	}

	src := withPagination{Name: "test", Limit: 50, Offset: 100}
	dst := CopyFields[withoutPagination](src)

	assert.Equal(t, "test", dst.Name)
	assert.Equal(t, int32(0), dst.Limit)  // skipped, zero value
	assert.Equal(t, int32(0), dst.Offset) // skipped, zero value
}

func TestCopyFields_MismatchedTypes_Skipped(t *testing.T) {
	type source struct {
		Name  string
		Value int32
	}
	type target struct {
		Name  string
		Value string // different type — should be skipped
	}

	src := source{Name: "hello", Value: 42}
	dst := CopyFields[target](src)

	assert.Equal(t, "hello", dst.Name)
	assert.Equal(t, "", dst.Value) // skipped, zero value (type mismatch)
}

func TestCopyFields_ExtraFields_Ignored(t *testing.T) {
	type source struct {
		Name  string
		Email string
		Age   int
	}
	type target struct {
		Name string
	}

	src := source{Name: "john", Email: "a@b.com", Age: 30}
	dst := CopyFields[target](src)

	assert.Equal(t, "john", dst.Name)
}

func TestCopyFields_EmptySource(t *testing.T) {
	src := listUsersParams{}
	count := CopyFields[countUsersParams](src)

	assert.Equal(t, pgtype.Text{}, count.Search)
	assert.Equal(t, pgtype.Text{}, count.Role)
}

func TestCopyFields_PointerSource(t *testing.T) {
	src := &listUsersParams{
		Search: pgtype.Text{String: "ptr", Valid: true},
		Limit:  10,
	}
	count := CopyFields[countUsersParams](src)

	assert.Equal(t, pgtype.Text{String: "ptr", Valid: true}, count.Search)
}

func TestCopyFields_NilPointerSource(t *testing.T) {
	var src *listUsersParams
	count := CopyFields[countUsersParams](src)

	assert.Equal(t, countUsersParams{}, count)
}

func TestCopyFields_Cached(t *testing.T) {
	// Call twice — second call should use cache
	src := listUsersParams{Search: pgtype.Text{String: "a", Valid: true}}

	count1 := CopyFields[countUsersParams](src)
	count2 := CopyFields[countUsersParams](src)

	assert.Equal(t, count1, count2)
}
