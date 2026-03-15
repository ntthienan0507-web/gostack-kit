package apperror

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// AppError is a structured error with an i18n-friendly message key.
//
//   - Code:    HTTP status code
//   - Message: snake_case key namespaced by module (e.g. "user.not_found")
//   - Detail:  human-readable description for debugging
type AppError struct {
	Code    int    `json:"error_code"`
	Message string `json:"error_message"`
	Detail  string `json:"error_detail"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Message, e.Detail)
}

// New creates an AppError.
func New(code int, message, detail string) *AppError {
	return &AppError{Code: code, Message: message, Detail: detail}
}

// WithDetail returns a copy of the AppError with a custom detail message.
// Useful for reusing a predefined error key with context-specific details.
func (e *AppError) WithDetail(detail string) *AppError {
	return &AppError{Code: e.Code, Message: e.Message, Detail: detail}
}

// Abort writes the AppError as JSON and aborts the gin chain.
func Abort(ctx *gin.Context, err *AppError) {
	ctx.AbortWithStatusJSON(err.Code, err)
}

// Respond writes the AppError as JSON without aborting.
func Respond(ctx *gin.Context, err *AppError) {
	ctx.JSON(err.Code, err)
}

// FromError inspects err and returns a matching AppError.
// Handles: *AppError, pgx, pgconn, MongoDB, and fallback to 500.
func FromError(err error) *AppError {
	if err == nil {
		return nil
	}

	// Already an AppError — return as-is
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	// pgx: no rows
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRecordNotFound
	}

	// pgconn: postgres constraint violations
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrRecordAlreadyExists
		case "23503":
			return ErrRelatedRecordNotFound
		case "23502":
			return ErrRequiredFieldMissing
		}
	}

	// MongoDB: duplicate key
	if mongo.IsDuplicateKeyError(err) {
		return ErrRecordAlreadyExists
	}

	return ErrInternalError
}

// HandleError is a convenience: converts err via FromError and writes the response.
func HandleError(ctx *gin.Context, err error) {
	Respond(ctx, FromError(err))
}
