package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	requestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
)

// RequestID ensures every request carries a unique identifier.
// If the incoming request has an X-Request-ID header, it is reused
// (useful for distributed tracing); otherwise a new UUID is generated.
// The ID is stored in the gin context and echoed back in the response header.
func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}

		ctx.Set(requestIDKey, id)
		ctx.Header(requestIDHeader, id)
		ctx.Next()
	}
}

// GetRequestID retrieves the request ID from context (set by RequestID middleware).
func GetRequestID(ctx *gin.Context) string {
	v, _ := ctx.Get(requestIDKey)
	id, _ := v.(string)
	return id
}
