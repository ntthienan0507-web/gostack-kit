package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCreateReq struct {
	Name  string `json:"name" binding:"required,min=3"`
	Email string `json:"email" binding:"required,email"`
	Age   int    `json:"age" binding:"required,min=1,max=150"`
}

func setupValidationRouter(template any) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/test", ValidateJSON(template), func(ctx *gin.Context) {
		req := GetBody[testCreateReq](ctx)
		if req == nil {
			ctx.JSON(500, gin.H{"error": "body not found"})
			return
		}
		ctx.JSON(200, gin.H{"name": req.Name, "email": req.Email})
	})
	return r
}

func TestValidateJSON_Success(t *testing.T) {
	r := setupValidationRouter(&testCreateReq{})

	body, _ := json.Marshal(testCreateReq{Name: "John", Email: "john@example.com", Age: 25})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "John")
	assert.Contains(t, w.Body.String(), "john@example.com")
}

func TestValidateJSON_EmptyBody(t *testing.T) {
	r := setupValidationRouter(&testCreateReq{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "body is required")
}

func TestValidateJSON_InvalidJSON(t *testing.T) {
	r := setupValidationRouter(&testCreateReq{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "validation_failed")
}

func TestValidateJSON_MissingRequiredField(t *testing.T) {
	r := setupValidationRouter(&testCreateReq{})

	body, _ := json.Marshal(map[string]any{"name": "Jo"}) // too short + missing email + age
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "validation_failed")
}

func TestValidateJSON_InvalidEmail(t *testing.T) {
	r := setupValidationRouter(&testCreateReq{})

	body, _ := json.Marshal(map[string]any{"name": "John", "email": "not-an-email", "age": 25})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestValidateJSON_HandlerNotReached(t *testing.T) {
	handlerCalled := false
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/test", ValidateJSON(&testCreateReq{}), func(ctx *gin.Context) {
		handlerCalled = true
		ctx.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "handler should NOT be called on validation failure")
}

func TestValidateQuery_Success(t *testing.T) {
	type listParams struct {
		Page     int    `form:"page" binding:"min=1"`
		PageSize int    `form:"page_size" binding:"min=1,max=100"`
		Search   string `form:"q"`
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/test", ValidateQuery(&listParams{}), func(ctx *gin.Context) {
		params := GetBody[listParams](ctx)
		require.NotNil(t, params)
		ctx.JSON(200, gin.H{"page": params.Page, "q": params.Search})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?page=2&page_size=10&q=hello", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"page":2`)
	assert.Contains(t, w.Body.String(), `"q":"hello"`)
}

func TestGetBody_WrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/test", ValidateJSON(&testCreateReq{}), func(ctx *gin.Context) {
		type wrongType struct{ Foo string }
		result := GetBody[wrongType](ctx)
		assert.Nil(t, result, "wrong type should return nil")
		ctx.Status(200)
	})

	body, _ := json.Marshal(testCreateReq{Name: "John", Email: "john@example.com", Age: 25})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
}
