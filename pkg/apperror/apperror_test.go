package apperror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

// --- AppError ---

func TestAppError_Error(t *testing.T) {
	err := New(400, "user.invalid_id", "The ID is not a valid UUID")

	assert.Equal(t, "user.invalid_id: The ID is not a valid UUID", err.Error())
}

func TestNew(t *testing.T) {
	err := New(404, "user.not_found", "User does not exist")

	assert.Equal(t, 404, err.Code)
	assert.Equal(t, "user.not_found", err.Message)
	assert.Equal(t, "User does not exist", err.Detail)
}

func TestWithDetail(t *testing.T) {
	original := New(400, "common.bad_request", "Invalid request")
	custom := original.WithDetail("Missing field: email")

	assert.Equal(t, original.Code, custom.Code)
	assert.Equal(t, original.Message, custom.Message)
	assert.Equal(t, "Missing field: email", custom.Detail)
	assert.Equal(t, "Invalid request", original.Detail)
}

// --- FromError ---

func TestFromError_Nil(t *testing.T) {
	result := FromError(nil)
	assert.Nil(t, result)
}

func TestFromError_AlreadyAppError(t *testing.T) {
	original := New(409, "user.already_exists", "Duplicate user")
	result := FromError(original)

	assert.Equal(t, original, result)
}

func TestFromError_WrappedAppError(t *testing.T) {
	original := New(409, "user.already_exists", "Duplicate user")
	wrapped := fmt.Errorf("repo layer: %w", original)
	result := FromError(wrapped)

	assert.Equal(t, original, result)
}

func TestFromError_PgxNoRows(t *testing.T) {
	result := FromError(pgx.ErrNoRows)

	assert.Equal(t, ErrRecordNotFound, result)
}

func TestFromError_PgUniqueViolation(t *testing.T) {
	result := FromError(&pgconn.PgError{Code: "23505"})

	assert.Equal(t, ErrRecordAlreadyExists, result)
}

func TestFromError_PgForeignKeyViolation(t *testing.T) {
	result := FromError(&pgconn.PgError{Code: "23503"})

	assert.Equal(t, ErrRelatedRecordNotFound, result)
}

func TestFromError_PgNotNullViolation(t *testing.T) {
	result := FromError(&pgconn.PgError{Code: "23502"})

	assert.Equal(t, ErrRequiredFieldMissing, result)
}

func TestFromError_UnknownPgError(t *testing.T) {
	result := FromError(&pgconn.PgError{Code: "42000"})

	assert.Equal(t, ErrInternalError, result)
}

func TestFromError_UnknownError(t *testing.T) {
	result := FromError(errors.New("something unexpected"))

	assert.Equal(t, ErrInternalError, result)
}

// --- Common errors ---

func TestCommonErrors_StatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected int
	}{
		{"bad request", ErrBadRequest, 400},
		{"invalid params", ErrInvalidParams, 400},
		{"validation failed", ErrValidationFailed, 400},
		{"required field", ErrRequiredFieldMissing, 400},
		{"unauthorized", ErrUnauthorized, 401},
		{"token missing", ErrTokenMissing, 401},
		{"token invalid", ErrTokenInvalid, 401},
		{"forbidden", ErrForbidden, 403},
		{"not found", ErrRecordNotFound, 404},
		{"route not found", ErrRouteNotFound, 404},
		{"conflict", ErrRecordAlreadyExists, 409},
		{"related not found", ErrRelatedRecordNotFound, 422},
		{"internal", ErrInternalError, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Code)
		})
	}
}
