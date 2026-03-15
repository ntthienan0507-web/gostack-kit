package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Response is the generic success envelope.
type Response[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

// ListData wraps paginated results.
type ListData[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

// OK sends 200 with typed response.
func OK[T any](ctx *gin.Context, data T) {
	ctx.JSON(http.StatusOK, Response[T]{Status: "success", Data: data})
}

// OKList sends 200 with typed list response.
func OKList[T any](ctx *gin.Context, items []T, total int64) {
	ctx.JSON(http.StatusOK, Response[ListData[T]]{Status: "success", Data: ListData[T]{Items: items, Total: total}})
}

// Success sends 200 (backward compat).
func Success(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Response[any]{Status: "success", Data: data})
}

// Created sends 201 with typed response.
func Created[T any](ctx *gin.Context, data T) {
	ctx.JSON(http.StatusCreated, Response[T]{Status: "success", Data: data})
}

// NoContent sends 204.
func NoContent(ctx *gin.Context) {
	ctx.Status(http.StatusNoContent)
}

// ErrorBody is the unified error response structure.
type ErrorBody struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func BadRequest(ctx *gin.Context, message string)    { ctx.JSON(http.StatusBadRequest, ErrorBody{Status: "error", Message: message}) }
func Unauthorized(ctx *gin.Context, message string)  { ctx.JSON(http.StatusUnauthorized, ErrorBody{Status: "error", Message: message}) }
func Forbidden(ctx *gin.Context, message string)     { ctx.JSON(http.StatusForbidden, ErrorBody{Status: "error", Message: message}) }
func NotFound(ctx *gin.Context, message string)      { ctx.JSON(http.StatusNotFound, ErrorBody{Status: "error", Message: message}) }
func Conflict(ctx *gin.Context, message string)      { ctx.JSON(http.StatusConflict, ErrorBody{Status: "error", Message: message}) }

func InternalError(ctx *gin.Context) {
	ctx.JSON(http.StatusInternalServerError, ErrorBody{Status: "error", Message: "An internal error occurred"})
}

func DBError(ctx *gin.Context, err error) {
	if err == nil { return }
	if errors.Is(err, pgx.ErrNoRows) { NotFound(ctx, "Record not found"); return }
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": Conflict(ctx, "Record already exists")
		case "23503": ctx.JSON(http.StatusUnprocessableEntity, ErrorBody{Status: "error", Message: "Related record not found"})
		case "23502": BadRequest(ctx, "Required field missing")
		default: InternalError(ctx)
		}
		return
	}
	InternalError(ctx)
}
