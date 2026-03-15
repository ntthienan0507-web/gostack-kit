package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
