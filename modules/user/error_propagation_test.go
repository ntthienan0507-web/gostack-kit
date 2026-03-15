package user

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// Tests verify that errors originating in the repository propagate through
// service → controller → HTTP response with the correct status code and
// error_message. This catches cases where wrapping with fmt.Errorf loses
// the %w verb, silently converting everything to 500.

func setupPropagationRouter() (*gin.Engine, *MockRepository) {
	gin.SetMode(gin.TestMode)
	repo := new(MockRepository)
	svc := NewService(repo, zap.NewNop())
	ctrl := NewController(svc, zap.NewNop())
	r := gin.New()
	r.GET("/users/:id", ctrl.GetByID)
	r.DELETE("/users/:id", ctrl.Delete)
	return r, repo
}

type errorCase struct {
	name           string
	repoError      error
	expectedStatus int
	expectedMsg    string
}

func propagationCases() []errorCase {
	return []errorCase{
		{
			name:           "pgx.ErrNoRows → 404",
			repoError:      pgx.ErrNoRows,
			expectedStatus: http.StatusNotFound,
			expectedMsg:    "common.record_not_found",
		},
		{
			name:           "wrapped pgx.ErrNoRows → 404",
			repoError:      fmt.Errorf("get user: %w", pgx.ErrNoRows),
			expectedStatus: http.StatusNotFound,
			expectedMsg:    "common.record_not_found",
		},
		{
			name: "pgconn unique violation → 409",
			repoError: &pgconn.PgError{
				Code:    "23505",
				Message: "duplicate key value violates unique constraint",
			},
			expectedStatus: http.StatusConflict,
			expectedMsg:    "common.record_already_exists",
		},
		{
			name: "wrapped pgconn unique violation → 409",
			repoError: fmt.Errorf("create user: %w", &pgconn.PgError{
				Code:    "23505",
				Message: "duplicate key",
			}),
			expectedStatus: http.StatusConflict,
			expectedMsg:    "common.record_already_exists",
		},
		{
			name: "pgconn FK violation → 422",
			repoError: &pgconn.PgError{
				Code:    "23503",
				Message: "foreign key constraint",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedMsg:    "common.related_record_not_found",
		},
		{
			name:           "AppError passthrough → original code",
			repoError:      ErrUserNotFound,
			expectedStatus: http.StatusNotFound,
			expectedMsg:    "user.not_found",
		},
		{
			name:           "wrapped AppError → original code",
			repoError:      fmt.Errorf("repo layer: %w", ErrUserNotFound),
			expectedStatus: http.StatusNotFound,
			expectedMsg:    "user.not_found",
		},
		{
			name:           "plain error → 500",
			repoError:      fmt.Errorf("connection refused"),
			expectedStatus: http.StatusInternalServerError,
			expectedMsg:    "common.internal_error",
		},
	}
}

func TestErrorPropagation_GetByID(t *testing.T) {
	for _, tc := range propagationCases() {
		t.Run(tc.name, func(t *testing.T) {
			router, repo := setupPropagationRouter()
			id := uuid.New()
			repo.On("GetByID", mock.Anything, id).Return(nil, tc.repoError)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/users/"+id.String(), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			var body apperror.AppError
			err := json.Unmarshal(w.Body.Bytes(), &body)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedMsg, body.Message)
		})
	}
}

func TestErrorPropagation_Delete_GetByIDPhase(t *testing.T) {
	for _, tc := range propagationCases() {
		t.Run("delete_lookup/"+tc.name, func(t *testing.T) {
			router, repo := setupPropagationRouter()
			id := uuid.New()
			repo.On("GetByID", mock.Anything, id).Return(nil, tc.repoError)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", "/users/"+id.String(), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			var body apperror.AppError
			err := json.Unmarshal(w.Body.Bytes(), &body)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedMsg, body.Message)
		})
	}
}
