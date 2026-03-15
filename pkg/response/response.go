package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrorBody is the unified error response structure.
type ErrorBody struct {
	Code    int    `json:"error"`
	Message string `json:"error_description"`
}

// Success sends 200 with auto-detection of paginated vs single result.
func Success(ctx *gin.Context, data interface{}) {
	switch v := data.(type) {
	case *PaginatedResult:
		ctx.JSON(http.StatusOK, gin.H{
			"message":    "Data successfully retrieved",
			"results":    v.Items,
			"pagination": v.Pagination,
		})
	default:
		ctx.JSON(http.StatusOK, gin.H{
			"message": "Data successfully retrieved",
			"result":  data,
		})
	}
}

// Created sends 201.
func Created(ctx *gin.Context, data interface{}) {
	ctx.JSON(http.StatusCreated, gin.H{
		"message": "Data successfully created",
		"result":  data,
	})
}

// NoContent sends 204 (successful delete).
func NoContent(ctx *gin.Context) {
	ctx.Status(http.StatusNoContent)
}

// BadRequest sends 400.
func BadRequest(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusBadRequest, ErrorBody{Code: 400, Message: message})
}

// Unauthorized sends 401.
func Unauthorized(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusUnauthorized, ErrorBody{Code: 401, Message: message})
}

// Forbidden sends 403.
func Forbidden(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusForbidden, ErrorBody{Code: 403, Message: message})
}

// NotFound sends 404.
func NotFound(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusNotFound, ErrorBody{Code: 404, Message: message})
}

// InternalError sends 500. Does NOT expose err details to client.
func InternalError(ctx *gin.Context) {
	ctx.JSON(http.StatusInternalServerError, ErrorBody{
		Code:    500,
		Message: "An internal error occurred",
	})
}

// DBError maps pgx/pgconn errors to appropriate HTTP responses.
func DBError(ctx *gin.Context, err error) {
	if err == nil {
		return
	}

	if errors.Is(err, pgx.ErrNoRows) {
		NotFound(ctx, "Record not found")
		return
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			ctx.JSON(http.StatusConflict, ErrorBody{Code: 409, Message: "Record already exists"})
		case "23503": // foreign_key_violation
			ctx.JSON(http.StatusUnprocessableEntity, ErrorBody{Code: 422, Message: "Related record not found"})
		case "23502": // not_null_violation
			BadRequest(ctx, "Required field missing")
		default:
			InternalError(ctx)
		}
		return
	}

	InternalError(ctx)
}
