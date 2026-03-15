package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func setupTestTracerProvider() func() {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	return func() { _ = tp.Shutdown(nil) }
}

func TestTracing_DoesNotBreakRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Tracing("test-service"))
	r.GET("/test", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestGetTraceID_ReturnsEmptyWithoutMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var traceID string
	r.GET("/test", func(ctx *gin.Context) {
		traceID = GetTraceID(ctx)
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, traceID)
}

func TestGetTraceID_ReturnsValueWithMiddleware(t *testing.T) {
	cleanup := setupTestTracerProvider()
	defer cleanup()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Tracing("test-service"), TracingIDs())

	var traceID string
	r.GET("/test", func(ctx *gin.Context) {
		traceID = GetTraceID(ctx)
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// With the tracing middleware active, a trace ID should be present
	assert.NotEmpty(t, traceID)
	// OpenTelemetry trace IDs are 32 hex characters
	assert.Len(t, traceID, 32)
}

func TestGetSpanID_ReturnsValueWithMiddleware(t *testing.T) {
	cleanup := setupTestTracerProvider()
	defer cleanup()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Tracing("test-service"), TracingIDs())

	var spanID string
	r.GET("/test", func(ctx *gin.Context) {
		spanID = GetSpanID(ctx)
		ctx.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, spanID)
	// OpenTelemetry span IDs are 16 hex characters
	assert.Len(t, spanID, 16)
}
