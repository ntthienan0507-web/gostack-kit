package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery catches panics, logs with zap, returns 500.
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					zap.Any("error", r),
					zap.String("path", ctx.Request.URL.Path),
				)
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":             500,
					"error_description": "Internal server error",
				})
			}
		}()
		ctx.Next()
	}
}
