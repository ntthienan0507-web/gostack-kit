package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// --- APIVersion middleware ---

func TestAPIVersion_FromURLPrefix(t *testing.T) {
	r := gin.New()
	r.Use(APIVersion("v1"))
	r.GET("/api/v2/users", func(ctx *gin.Context) {
		ctx.String(200, GetAPIVersion(ctx))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/users", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v2", w.Body.String())
}

func TestAPIVersion_FromHeader(t *testing.T) {
	r := gin.New()
	r.Use(APIVersion("v1"))
	r.GET("/users", func(ctx *gin.Context) {
		ctx.String(200, GetAPIVersion(ctx))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users", nil)
	req.Header.Set("X-API-Version", "v2")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v2", w.Body.String())
}

func TestAPIVersion_FromQueryParam(t *testing.T) {
	r := gin.New()
	r.Use(APIVersion("v1"))
	r.GET("/users", func(ctx *gin.Context) {
		ctx.String(200, GetAPIVersion(ctx))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users?api_version=v3", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v3", w.Body.String())
}

func TestAPIVersion_Default(t *testing.T) {
	r := gin.New()
	r.Use(APIVersion("v1"))
	r.GET("/users", func(ctx *gin.Context) {
		ctx.String(200, GetAPIVersion(ctx))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v1", w.Body.String())
}

func TestAPIVersion_URLTakesPrecedenceOverHeader(t *testing.T) {
	r := gin.New()
	r.Use(APIVersion("v1"))
	r.GET("/api/v2/users", func(ctx *gin.Context) {
		ctx.String(200, GetAPIVersion(ctx))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v2/users", nil)
	req.Header.Set("X-API-Version", "v3")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v2", w.Body.String())
}

func TestAPIVersion_HeaderTakesPrecedenceOverQuery(t *testing.T) {
	r := gin.New()
	r.Use(APIVersion("v1"))
	r.GET("/users", func(ctx *gin.Context) {
		ctx.String(200, GetAPIVersion(ctx))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users?api_version=v3", nil)
	req.Header.Set("X-API-Version", "v2")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "v2", w.Body.String())
}

// --- GetAPIVersion without middleware ---

func TestGetAPIVersion_EmptyWhenNotSet(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(ctx *gin.Context) {
		ctx.String(200, GetAPIVersion(ctx))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Body.String())
}

// --- DeprecatedVersion middleware ---

func TestDeprecatedVersion_AddsSunsetHeader(t *testing.T) {
	r := gin.New()
	r.Use(DeprecatedVersion("2025-06-01", "Use /api/v2 instead"))
	r.GET("/api/v1/users", func(ctx *gin.Context) {
		ctx.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get("Deprecation"))
	assert.Equal(t, "2025-06-01", w.Header().Get("Sunset"))
	assert.Contains(t, w.Header().Get("Link"), "Use /api/v2 instead")
}

func TestDeprecatedVersion_DoesNotBreakRequest(t *testing.T) {
	r := gin.New()
	r.Use(DeprecatedVersion("2025-12-31", "Migrate to v3"))
	r.GET("/test", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}
