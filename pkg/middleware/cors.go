package middleware

import "github.com/gin-gonic/gin"

// CORS adds Cross-Origin Resource Sharing headers.
func CORS(allowedOrigins string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", allowedOrigins)
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, Authorization, Cache-Control")
		ctx.Header("Access-Control-Allow-Methods",
			"POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(204)
			return
		}
		ctx.Next()
	}
}
