package middleware

import (
	"errors"
	"io"
	"reflect"

	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

const validatedRequestKey = "validated_request"

// ValidateJSON validates the request body against the given struct type.
// If valid, stores the parsed struct in context — handler retrieves via GetBody[T].
// If invalid (no body, bad JSON, validation errors), aborts with AppError.
//
// Usage:
//
//	// In routes.go — validate BEFORE handler runs
//	users.POST("", middleware.ValidateJSON(&CreateRequest{}), ctrl.Create)
//
//	// In controller — body is guaranteed valid, no EOF
//	func (c *Controller) Create(ctx *gin.Context) {
//	    req := middleware.GetBody[CreateRequest](ctx)
//	    user, err := c.service.Create(ctx.Request.Context(), *req)
//	    ...
//	}
func ValidateJSON(template any) gin.HandlerFunc {
	// Resolve the element type at registration time (once, not per request)
	reqType := reflect.TypeOf(template)
	if reqType.Kind() == reflect.Ptr {
		reqType = reqType.Elem()
	}

	return func(ctx *gin.Context) {
		// No body at all
		if ctx.Request.Body == nil || ctx.Request.ContentLength == 0 {
			abortWithAppError(ctx, apperror.ErrBadRequest.WithDetail("Request body is required"))
			return
		}

		// Create a new zero-value instance of the struct
		req := reflect.New(reqType).Interface()

		if err := ctx.ShouldBindJSON(req); err != nil {
			// Empty body → EOF (e.g. Content-Length > 0 but body is empty)
			if errors.Is(err, io.EOF) {
				abortWithAppError(ctx, apperror.ErrBadRequest.WithDetail("Request body is required"))
				return
			}

			// Validation error (binding tags: required, min, max, email, etc.)
			abortWithAppError(ctx, apperror.ErrValidationFailed.WithDetail(err.Error()))
			return
		}

		ctx.Set(validatedRequestKey, req)
		ctx.Next()
	}
}

// ValidateQuery validates query parameters against the given struct type.
// Same pattern as ValidateJSON but for GET requests with query params.
//
// Usage:
//
//	users.GET("", middleware.ValidateQuery(&ListParams{}), ctrl.List)
//
//	func (c *Controller) List(ctx *gin.Context) {
//	    params := middleware.GetBody[ListParams](ctx)
//	    ...
//	}
func ValidateQuery(template any) gin.HandlerFunc {
	reqType := reflect.TypeOf(template)
	if reqType.Kind() == reflect.Ptr {
		reqType = reqType.Elem()
	}

	return func(ctx *gin.Context) {
		req := reflect.New(reqType).Interface()

		if err := ctx.ShouldBindQuery(req); err != nil {
			abortWithAppError(ctx, apperror.ErrInvalidParams.WithDetail(err.Error()))
			return
		}

		ctx.Set(validatedRequestKey, req)
		ctx.Next()
	}
}

// GetBody retrieves the validated request body from context.
// Returns nil if not found or type mismatch.
// The body is guaranteed to be valid if ValidateJSON/ValidateQuery middleware ran.
func GetBody[T any](ctx *gin.Context) *T {
	val, exists := ctx.Get(validatedRequestKey)
	if !exists {
		return nil
	}
	if req, ok := val.(*T); ok {
		return req
	}
	return nil
}
