package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPinger implements DBPinger for testing.
type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(_ context.Context) error {
	return m.err
}

func setupTestRouter(pinger DBPinger) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/readyz", func(c *gin.Context) {
		ReadinessCheck(c, pinger, nil) // no Redis in tests
	})
	return r
}

func TestReadinessCheck_Healthy(t *testing.T) {
	r := setupTestRouter(&mockPinger{err: nil})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ready", body["status"])

	checks := body["checks"].(map[string]any)
	assert.Equal(t, "ok", checks["database"])
	assert.Equal(t, "skipped", checks["redis"])
}

func TestReadinessCheck_Unhealthy(t *testing.T) {
	r := setupTestRouter(&mockPinger{err: errors.New("connection refused")})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "not_ready", body["status"])

	checks := body["checks"].(map[string]any)
	assert.Contains(t, checks["database"], "failed: connection refused")
}

func TestReadinessCheck_NoDB(t *testing.T) {
	r := setupTestRouter(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
	r.ServeHTTP(w, req)

	// No DB + no Redis = still 200 (both skipped, not failed)
	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ready", body["status"])

	checks := body["checks"].(map[string]any)
	assert.Equal(t, "skipped", checks["database"])
	assert.Equal(t, "skipped", checks["redis"])
}
