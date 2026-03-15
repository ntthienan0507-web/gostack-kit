package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ntthienan0507-web/go-api-template/pkg/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockProvider implements auth.Provider for testing.
type mockProvider struct {
	validateFn func(ctx context.Context, token string) (*auth.Claims, error)
}

func (m *mockProvider) ValidateToken(ctx context.Context, token string) (*auth.Claims, error) {
	return m.validateFn(ctx, token)
}

func (m *mockProvider) GenerateToken(_, _, _ string) (string, error) { return "", nil }
func (m *mockProvider) RefreshToken(_ context.Context, _ string) (string, error) {
	return "", nil
}

// --- Auth middleware ---

func TestAuth_ValidToken(t *testing.T) {
	provider := &mockProvider{
		validateFn: func(_ context.Context, token string) (*auth.Claims, error) {
			return &auth.Claims{UserID: "user-1", Email: "a@b.com", Role: "admin"}, nil
		},
	}

	r := gin.New()
	r.Use(Auth(provider))
	r.GET("/test", func(ctx *gin.Context) {
		claims, ok := GetClaims(ctx)
		require.True(t, ok)
		ctx.JSON(200, gin.H{"user_id": claims.UserID, "role": claims.Role})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "user-1")
}

func TestAuth_MissingHeader(t *testing.T) {
	provider := &mockProvider{}
	r := gin.New()
	r.Use(Auth(provider))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Missing bearer token")
}

func TestAuth_MissingBearerPrefix(t *testing.T) {
	provider := &mockProvider{}
	r := gin.New()
	r.Use(Auth(provider))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic abc123")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidToken(t *testing.T) {
	provider := &mockProvider{
		validateFn: func(_ context.Context, _ string) (*auth.Claims, error) {
			return nil, errors.New("expired")
		},
	}

	r := gin.New()
	r.Use(Auth(provider))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid or expired token")
}

// --- GetClaims ---

func TestGetClaims_NoClaims(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	claims, ok := GetClaims(ctx)

	assert.False(t, ok)
	assert.Nil(t, claims)
}

func TestGetClaims_WithClaims(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	expected := &auth.Claims{UserID: "u1", Role: "admin"}
	ctx.Set("claims", expected)

	claims, ok := GetClaims(ctx)

	assert.True(t, ok)
	assert.Equal(t, expected, claims)
}

// --- RequireRole ---

func TestRequireRole_Allowed(t *testing.T) {
	r := gin.New()
	r.Use(func(ctx *gin.Context) {
		ctx.Set("claims", &auth.Claims{UserID: "u1", Role: "admin"})
		ctx.Next()
	})
	r.Use(RequireRole("admin", "superadmin"))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Forbidden(t *testing.T) {
	r := gin.New()
	r.Use(func(ctx *gin.Context) {
		ctx.Set("claims", &auth.Claims{UserID: "u1", Role: "user"})
		ctx.Next()
	})
	r.Use(RequireRole("admin"))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient permissions")
}

func TestRequireRole_NoClaims(t *testing.T) {
	r := gin.New()
	r.Use(RequireRole("admin"))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireRole_MultipleRoles(t *testing.T) {
	for _, role := range []string{"admin", "moderator"} {
		t.Run(role, func(t *testing.T) {
			r := gin.New()
			r.Use(func(ctx *gin.Context) {
				ctx.Set("claims", &auth.Claims{UserID: "u1", Role: role})
				ctx.Next()
			})
			r.Use(RequireRole("admin", "moderator"))
			r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
