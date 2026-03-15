package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestID_AutoGenerate(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(w, req)

	id := w.Header().Get(requestIDHeader)
	assert.NotEmpty(t, id)

	// Must be a valid UUID.
	_, err := uuid.Parse(id)
	require.NoError(t, err)
}

func TestRequestID_ForwardExisting(t *testing.T) {
	existing := "trace-abc-123"

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(requestIDHeader, existing)
	router.ServeHTTP(w, req)

	assert.Equal(t, existing, w.Header().Get(requestIDHeader))
}

func TestGetRequestID(t *testing.T) {
	var captured string

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(ctx *gin.Context) {
		captured = GetRequestID(ctx)
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(w, req)

	assert.NotEmpty(t, captured)
	assert.Equal(t, w.Header().Get(requestIDHeader), captured)
}

func TestGetRequestID_WithoutMiddleware(t *testing.T) {
	var captured string

	router := gin.New()
	router.GET("/ping", func(ctx *gin.Context) {
		captured = GetRequestID(ctx)
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(w, req)

	assert.Empty(t, captured)
	_ = w // suppress unused
}

func TestRequestID_ResponseHeader(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get(requestIDHeader))
}
