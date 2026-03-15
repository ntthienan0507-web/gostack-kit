package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_PassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/ok", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ok", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestMetrics_EndpointReturnsMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/test", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Hit /test to generate metrics
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	// Now scrape /metrics
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "http_requests_total")
	assert.Contains(t, body, "http_request_duration_seconds")
	assert.Contains(t, body, "http_requests_in_flight")
}

func TestNormalizePath_UUID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/api/v1/users/550e8400-e29b-41d4-a716-446655440000",
			expected: "/api/v1/users/:id",
		},
		{
			input:    "/api/v1/users/550e8400-e29b-41d4-a716-446655440000/posts/6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			expected: "/api/v1/users/:id/posts/:id",
		},
		{
			input:    "/api/v1/users/123",
			expected: "/api/v1/users/:id",
		},
		{
			input:    "/api/v1/healthz",
			expected: "/api/v1/healthz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
