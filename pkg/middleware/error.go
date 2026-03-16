package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// abortWithAppError writes the AppError as JSON and aborts the gin chain.
// Middleware uses this directly instead of going through pkg/response,
// keeping middleware independent of the response formatting layer.
func abortWithAppError(ctx *gin.Context, err *apperror.AppError) {
	ctx.AbortWithStatusJSON(err.Code, err)
}
