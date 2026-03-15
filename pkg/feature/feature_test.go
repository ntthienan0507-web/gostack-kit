package feature

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func testLogger() *zap.Logger {
	l, _ := zap.NewDevelopment()
	return l
}

func TestIsEnabled(t *testing.T) {
	fm := New(Flags{
		EnableWebSocket: true,
		EnableBetaAPI:   false,
		EnableKafka:     true,
		MaintenanceMode: false,
	}, testLogger())

	assert.True(t, fm.IsEnabled("websocket"))
	assert.False(t, fm.IsEnabled("beta_api"))
	assert.True(t, fm.IsEnabled("kafka"))
	assert.False(t, fm.IsEnabled("maintenance"))
	assert.False(t, fm.IsEnabled("unknown_flag"))
}

func TestAll(t *testing.T) {
	flags := Flags{
		EnableWebSocket: true,
		EnableBetaAPI:   true,
		EnableKafka:     false,
		MaintenanceMode: false,
	}
	fm := New(flags, testLogger())
	got := fm.All()
	assert.Equal(t, flags, got)
}

func TestUpdate(t *testing.T) {
	fm := New(Flags{}, testLogger())
	assert.False(t, fm.IsEnabled("beta_api"))

	fm.Update(Flags{EnableBetaAPI: true})
	assert.True(t, fm.IsEnabled("beta_api"))
}

func TestMaintenanceMiddleware_Blocks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fm := New(Flags{MaintenanceMode: true}, testLogger())

	w := httptest.NewRecorder()
	ctx, r := gin.CreateTestContext(w)
	r.Use(fm.MaintenanceMiddleware())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	ctx.Request = httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, ctx.Request)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "common.maintenance", body["error_message"])
}

func TestMaintenanceMiddleware_Passes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fm := New(Flags{MaintenanceMode: false}, testLogger())

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(fm.MaintenanceMiddleware())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireFlag_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fm := New(Flags{EnableBetaAPI: false}, testLogger())

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.GET("/beta", fm.RequireFlag("beta_api"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "beta"})
	})

	req := httptest.NewRequest(http.MethodGet, "/beta", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "common.feature_not_available", body["error_message"])
}

func TestRequireFlag_Enabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fm := New(Flags{EnableBetaAPI: true}, testLogger())

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.GET("/beta", fm.RequireFlag("beta_api"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "beta"})
	})

	req := httptest.NewRequest(http.MethodGet, "/beta", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
