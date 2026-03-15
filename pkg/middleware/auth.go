package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
	"github.com/ntthienan0507-web/gostack-kit/pkg/auth"
)

const claimsKey = "claims"

// Auth validates bearer tokens using the injected AuthProvider.
// No global state access — provider is injected via closure.
func Auth(provider auth.Provider) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			apperror.Abort(ctx, apperror.ErrTokenMissing)
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := provider.ValidateToken(ctx.Request.Context(), token)
		if err != nil {
			apperror.Abort(ctx, apperror.ErrTokenInvalid)
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
			apperror.Abort(ctx, apperror.ErrUnauthorized)
			return
		}

		if _, allowed := roleSet[claims.Role]; !allowed {
			apperror.Abort(ctx, apperror.ErrForbidden)
			return
		}

		ctx.Next()
	}
}
