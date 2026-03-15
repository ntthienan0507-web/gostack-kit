package order

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/pkg/response"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(ctrl *Controller) *gin.Engine {
	r := gin.New()
	g := r.Group("/api/v1")
	{
		g.GET("/orders", ctrl.List)
		g.POST("/orders", ctrl.Create)
		g.GET("/orders/:id", ctrl.GetByID)
		g.POST("/orders/:id/cancel", ctrl.Cancel)
	}
	return r
}

func newTestController() (*Controller, *MockRepository) {
	repo := new(MockRepository)
	logger := zap.NewNop()
	svc := NewService(repo, nil, nil, nil, logger)
	ctrl := NewController(svc, logger)
	return ctrl, repo
}

// --- List ---

func TestController_List_Success(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	orders := []*Order{sampleOrder(), sampleOrder()}
	repo.On("List", mock.Anything, mock.Anything, int32(20), int32(0)).Return(orders, nil)
	repo.On("Count", mock.Anything, mock.Anything).Return(int64(2), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body response.Response[response.ListData[OrderResponse]]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body.Status)
	assert.Len(t, body.Data.Items, 2)
	assert.Equal(t, int64(2), body.Data.Total)
}

func TestController_List_WithSearchAndStatus(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	repo.On("List", mock.Anything, mock.MatchedBy(func(p ListParams) bool {
		return p.Search == "rush" && p.Status == "pending"
	}), int32(10), int32(0)).Return([]*Order{}, nil)
	repo.On("Count", mock.Anything, mock.MatchedBy(func(p ListParams) bool {
		return p.Search == "rush" && p.Status == "pending"
	})).Return(int64(0), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders?q=rush&status=pending&page_size=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestController_List_ServiceError(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	repo.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("db failure"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GetByID ---

func TestController_GetByID_Success(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	o := sampleOrder()
	repo.On("GetByID", mock.Anything, o.ID).Return(o, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders/"+o.ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body response.Response[OrderResponse]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, o.ID, body.Data.ID)
	assert.Equal(t, o.UserID, body.Data.UserID)
}

func TestController_GetByID_InvalidUUID(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders/not-a-uuid", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_GetByID_NotFound(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, ErrOrderNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders/"+id.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Create ---

func TestController_Create_ValidationError_MissingBody(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_Create_ValidationError_EmptyItems(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":  uuid.New().String(),
		"currency": "USD",
		"items":    []interface{}{},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_Create_ValidationError_InvalidCurrency(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":  uuid.New().String(),
		"currency": "TOOLONG",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "name": "Widget", "quantity": 1, "unit_price": 9.99},
		},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_Create_ValidationError_InvalidItemQuantity(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":  uuid.New().String(),
		"currency": "USD",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "name": "Widget", "quantity": 0, "unit_price": 9.99},
		},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_Create_ValidationError_InvalidUnitPrice(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":  uuid.New().String(),
		"currency": "USD",
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "name": "Widget", "quantity": 1, "unit_price": 0},
		},
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Cancel ---

func TestController_Cancel_InvalidUUID(t *testing.T) {
	ctrl, _ := newTestController()
	router := setupRouter(ctrl)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders/bad-id/cancel", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestController_Cancel_NotFound(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, ErrOrderNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders/"+id.String()+"/cancel", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestController_Cancel_NotCancellable(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	o := sampleOrder()
	o.Status = StatusShipped
	repo.On("GetByID", mock.Anything, o.ID).Return(o, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/orders/"+o.ID.String()+"/cancel", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// Verify context propagation
func TestController_GetByID_UsesRequestContext(t *testing.T) {
	ctrl, repo := newTestController()
	router := setupRouter(ctrl)

	o := sampleOrder()
	repo.On("GetByID", mock.MatchedBy(func(ctx interface{}) bool {
		return ctx != nil
	}), o.ID).Return(o, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/orders/"+o.ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
