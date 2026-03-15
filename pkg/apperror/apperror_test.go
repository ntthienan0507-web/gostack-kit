package apperror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func performRequest(handler gin.HandlerFunc) *httptest.ResponseRecorder {
	r := gin.New()
	r.GET("/test", handler)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	return w
}

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

// --- Abort ---

func TestAbort(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(ctx *gin.Context) {
		Abort(ctx, ErrTokenMissing)
	}, func(ctx *gin.Context) {
		// This handler should not be reached
		ctx.JSON(200, gin.H{"should": "not appear"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body AppError
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "common.token_missing", body.Message)
}

// --- Respond ---

func TestRespond(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		Respond(ctx, ErrRecordNotFound)
	})

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body AppError
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "common.record_not_found", body.Message)
	assert.Equal(t, "Record not found", body.Detail)
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

// --- HandleError ---

func TestHandleError_PgxNoRows(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		HandleError(ctx, pgx.ErrNoRows)
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleError_UniqueViolation(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		HandleError(ctx, &pgconn.PgError{Code: "23505"})
	})

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandleError_GenericError(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		HandleError(ctx, errors.New("random"))
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
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
