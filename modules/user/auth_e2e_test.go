//go:build integration

package user_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	db "github.com/ntthienan0507-web/gostack-kit/db/sqlc"
	"github.com/ntthienan0507-web/gostack-kit/modules/user"
	"github.com/ntthienan0507-web/gostack-kit/pkg/auth"
	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/testutil"
)

func init() { gin.SetMode(gin.TestMode) }

func setupAuthRouter(t *testing.T) (*gin.Engine, auth.Provider) {
	t.Helper()

	pool := testutil.NewPostgresContainer(t)
	queries := db.New(pool)
	repo := user.NewSQLCRepository(queries)
	svc := user.NewService(repo, zap.NewNop())
	ctrl := user.NewController(svc, zap.NewNop())

	provider, err := auth.NewJWTProvider(&config.Config{
		JWTSecret: "test-secret-that-is-at-least-32-chars",
		JWTExpiry: time.Hour,
	})
	require.NoError(t, err)

	r := gin.New()
	api := r.Group("/api/v1")
	user.NewRoutes(ctrl).Register(api, provider)

	return r, provider
}

func TestAuthE2E_NoToken_Returns401(t *testing.T) {
	router, _ := setupAuthRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "common.token_missing", body["error_message"])
}

func TestAuthE2E_InvalidToken_Returns401(t *testing.T) {
	router, _ := setupAuthRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	assert.Equal(t, "common.token_invalid", body["error_message"])
}

func TestAuthE2E_ValidToken_ListUsers(t *testing.T) {
	router, provider := setupAuthRouter(t)

	token, err := provider.GenerateToken("user-1", "user@test.com", "user")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthE2E_AdminCreate_UserForbidden(t *testing.T) {
	router, provider := setupAuthRouter(t)

	// Non-admin token
	token, err := provider.GenerateToken("user-1", "user@test.com", "user")
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]string{
		"username":  "newuser",
		"email":     "new@test.com",
		"full_name": "New User",
		"password":  "password123",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "common.forbidden", resp["error_message"])
}

func TestAuthE2E_AdminCreate_Success(t *testing.T) {
	router, provider := setupAuthRouter(t)

	token, err := provider.GenerateToken("admin-1", "admin@test.com", "admin")
	require.NoError(t, err)

	body, _ := json.Marshal(map[string]string{
		"username":  "created_user",
		"email":     "created@test.com",
		"full_name": "Created User",
		"password":  "password123",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	assert.Equal(t, "created_user", data["username"])
	assert.Equal(t, "created@test.com", data["email"])
}

func TestAuthE2E_FullFlow_CreateGetUpdateDelete(t *testing.T) {
	router, provider := setupAuthRouter(t)

	adminToken, err := provider.GenerateToken("admin-1", "admin@test.com", "admin")
	require.NoError(t, err)

	// 1. Create user (admin only)
	createBody, _ := json.Marshal(map[string]string{
		"username":  "flow_user",
		"email":     "flow@test.com",
		"full_name": "Flow User",
		"password":  "password123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	userID := createResp["data"].(map[string]any)["id"].(string)

	// 2. Get user by ID (any authenticated user)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var getResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &getResp)
	assert.Equal(t, "flow_user", getResp["data"].(map[string]any)["username"])

	// 3. Update user
	updateBody, _ := json.Marshal(map[string]any{
		"full_name": "Updated Flow User",
	})
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PUT", "/api/v1/users/"+userID, bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var updateResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &updateResp)
	assert.Equal(t, "Updated Flow User", updateResp["data"].(map[string]any)["full_name"])

	// 4. Delete user (admin only)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("DELETE", "/api/v1/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// 5. Verify deleted — GET returns 404
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAuthE2E_TokenRefresh_Success(t *testing.T) {
	router, provider := setupAuthRouter(t)

	// Generate initial token
	token, err := provider.GenerateToken("user-1", "user@test.com", "user")
	require.NoError(t, err)

	// Refresh it
	refreshed, err := provider.RefreshToken(nil, token)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshed)
	assert.NotEqual(t, token, refreshed)

	// Use refreshed token to access API
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+refreshed)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
