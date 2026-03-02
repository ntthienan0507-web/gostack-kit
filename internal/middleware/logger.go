package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger logs each HTTP request using structured zap logging.
// No fmt.Println or fmt.Printf — structured only.
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path

		ctx.Next()

		logger.Info("request",
			zap.String("method", ctx.Request.Method),
			zap.String("path", path),
			zap.Int("status", ctx.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", ctx.ClientIP()),
		)
	}
}
