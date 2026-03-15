package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// --- Recovery middleware ---

func TestRecovery_NoPanic(t *testing.T) {
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/ok", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ok", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRecovery_CatchesPanic(t *testing.T) {
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/panic", func(ctx *gin.Context) {
		panic("something went wrong")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal server error")
}

// --- CORS middleware ---

func TestCORS_SetsHeaders(t *testing.T) {
	r := gin.New()
	r.Use(CORS("http://localhost:3000"))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestCORS_OptionsPreflightReturns204(t *testing.T) {
	r := gin.New()
	r.Use(CORS("*"))
	r.OPTIONS("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardOrigin(t *testing.T) {
	r := gin.New()
	r.Use(CORS("*"))
	r.GET("/test", func(ctx *gin.Context) { ctx.Status(200) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

// --- RequestLogger middleware ---

func TestRequestLogger_DoesNotBreakRequest(t *testing.T) {
	logger := zap.NewNop()
	r := gin.New()
	r.Use(RequestLogger(logger))
	r.GET("/test", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}
