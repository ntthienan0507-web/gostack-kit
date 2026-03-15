package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(ctrl *Controller) *gin.Engine {
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/users", ctrl.List)
		g.POST("/users", ctrl.Create)
		g.GET("/users/:id", ctrl.GetByID)
		g.PUT("/users/:id", ctrl.Update)
		g.DELETE("/users/:id", ctrl.Delete)
	}
	return r
}

func newTestController() (*Controller, *MockRepository) {
	repo := new(MockRepository)
	logger := zap.NewNop()
	svc := NewService(repo, logger)
	ctrl := NewController(svc, logger)
	return ctrl, repo
}

// --- List ---

func TestController_List_Success(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	users := []*User{sampleUser(), sampleUser()}
	repo.On("List", mock.Anything, mock.Anything, int32(20), int32(0)).Return(users, nil)
	repo.On("Count", mock.Anything, mock.Anything).Return(int64(2), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body response.Response[response.ListData[UserResponse]]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body.Status)
	assert.Len(t, body.Data.Items, 2)
	assert.Equal(t, int64(2), body.Data.Total)
}

func TestController_List_WithSearchAndRole(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	repo.On("List", mock.Anything, mock.MatchedBy(func(p ListParams) bool {
		return p.Search == "john" && p.Role == "admin"
	}), int32(10), int32(0)).Return([]*User{}, nil)
	repo.On("Count", mock.Anything, mock.MatchedBy(func(p ListParams) bool {
		return p.Search == "john" && p.Role == "admin"
	})).Return(int64(0), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users?q=john&role=admin&page_size=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestController_List_ServiceError(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	repo.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("db failure"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GetByID ---

func TestController_GetByID_Success(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	u := sampleUser()
	repo.On("GetByID", mock.Anything, u.ID).Return(u, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users/"+u.ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body response.Response[UserResponse]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, u.ID, body.Data.ID)
}

func TestController_GetByID_InvalidUUID(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users/not-a-uuid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_GetByID_NotFound(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, ErrUserNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users/"+id.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Create ---

func TestController_Create_Success(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	created := &User{
		ID:        uuid.New(),
		Username:  "newuser",
		Email:     "new@example.com",
		FullName:  "New User",
		Role:      "user",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.On("Create", mock.Anything, mock.Anything).Return(created, nil)

	body, _ := json.Marshal(CreateRequest{
		Username: "newuser",
		Email:    "new@example.com",
		FullName: "New User",
		Password: "securepassword",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestController_Create_ValidationError(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	tests := []struct {
		name string
		body CreateRequest
	}{
		{"missing username", CreateRequest{Email: "a@b.com", FullName: "A", Password: "12345678"}},
		{"missing email", CreateRequest{Username: "abc", FullName: "A", Password: "12345678"}},
		{"invalid email", CreateRequest{Username: "abc", Email: "not-email", FullName: "A", Password: "12345678"}},
		{"short password", CreateRequest{Username: "abc", Email: "a@b.com", FullName: "A", Password: "short"}},
		{"missing full_name", CreateRequest{Username: "abc", Email: "a@b.com", Password: "12345678"}},
		{"invalid role", CreateRequest{Username: "abc", Email: "a@b.com", FullName: "A", Password: "12345678", Role: "superadmin"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// --- Update ---

func TestController_Update_Success(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	u := sampleUser()
	newName := "Updated"
	repo.On("GetByID", mock.Anything, u.ID).Return(u, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(&User{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		FullName: newName,
		Role:     u.Role,
		IsActive: u.IsActive,
	}, nil)

	body, _ := json.Marshal(UpdateRequest{FullName: &newName})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/users/"+u.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestController_Update_InvalidUUID(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/users/bad-id", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Delete ---

func TestController_Delete_Success(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	u := sampleUser()
	repo.On("GetByID", mock.Anything, u.ID).Return(u, nil)
	repo.On("SoftDelete", mock.Anything, u.ID).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/users/"+u.ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestController_Delete_InvalidUUID(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/users/bad-id", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_Delete_NotFound(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, ErrUserNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/users/"+id.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Verify context propagation
func TestController_GetByID_UsesRequestContext(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	u := sampleUser()
	repo.On("GetByID", mock.MatchedBy(func(ctx context.Context) bool {
		return ctx != nil
	}), u.ID).Return(u, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/users/"+u.ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
