package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/go-api-template/pkg/auth"
	"github.com/ntthienan0507-web/go-api-template/pkg/response"
)

const claimsKey = "claims"

// Auth validates bearer tokens using the injected AuthProvider.
// No global state access — provider is injected via closure.
func Auth(provider auth.Provider) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(ctx, "Missing bearer token")
			ctx.Abort()
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := provider.ValidateToken(ctx.Request.Context(), token)
		if err != nil {
			response.Unauthorized(ctx, "Invalid or expired token")
			ctx.Abort()
			return
		}

		ctx.Set(claimsKey, claims)
		ctx.Next()
	}
}

// GetClaims retrieves auth Claims from context (set by Auth middleware).
func GetClaims(ctx *gin.Context) (*auth.Claims, bool) {
	v, exists := ctx.Get(claimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := v.(*auth.Claims)
	return claims, ok
}

// RequireRole checks that the authenticated user has the specified role.
func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		roleSet[r] = struct{}{}
	}

	return func(ctx *gin.Context) {
		claims, ok := GetClaims(ctx)
		if !ok {
			response.Unauthorized(ctx, "Authentication required")
			ctx.Abort()
			return
		}

		if _, allowed := roleSet[claims.Role]; !allowed {
			response.Forbidden(ctx, "Insufficient permissions")
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}
