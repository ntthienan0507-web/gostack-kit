package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout creates a middleware that sets a deadline on the request context.
// If the handler does not complete within the given duration, the context
// is cancelled — downstream code using ctx.Done() will be notified.
//
// Usage:
//
//	router.POST("/heavy", middleware.Timeout(30*time.Second), controller.HeavyOperation)
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// LongRunning is a preset for operations that need more time (5 minutes).
// Use for file uploads, report generation, batch imports, etc.
func LongRunning() gin.HandlerFunc {
	return Timeout(5 * time.Minute)
}
