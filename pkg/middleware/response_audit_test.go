package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
	"github.com/ntthienan0507-web/go-api-template/pkg/response"
)

func newObservedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.WarnLevel)
	return zap.New(core), logs
}

func auditRouter(logger *zap.Logger, handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(ResponseAudit(logger))
	r.GET("/test", handler)
	r.POST("/test", handler)
	return r
}

func doGet(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	return w
}

// --- Valid responses: no warnings ---

func TestAudit_ValidSuccess_NoWarning(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		response.OK(ctx, gin.H{"id": "123"})
	})

	w := doGet(r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, logs.Len(), "should not log warning for valid success response")
}

func TestAudit_ValidCreated_NoWarning(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		response.Created(ctx, gin.H{"id": "456"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", strings.NewReader("{}"))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, 0, logs.Len())
}

func TestAudit_ValidOKList_NoWarning(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		response.OKList(ctx, []string{"a", "b"}, 2)
	})

	w := doGet(r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, logs.Len())
}

func TestAudit_ValidAppError_NoWarning(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		apperror.Respond(ctx, apperror.ErrRecordNotFound)
	})

	w := doGet(r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, 0, logs.Len())
}

func TestAudit_NoContent_NoWarning(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		response.NoContent(ctx)
	})

	w := doGet(r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 0, logs.Len())
}

// --- Invalid responses: should warn ---

func TestAudit_HardcodedJSON_WarnsOnSuccess(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		// Junior developer hardcoding response
		ctx.JSON(200, gin.H{"message": "user created", "user": "john"})
	})

	w := doGet(r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, logs.Len(), "should log warning for non-standard response")
	assert.Contains(t, logs.All()[0].Message, "non-standard response format")
	assert.Contains(t, logs.All()[0].ContextMap()["hint"], "response.OK")
}

func TestAudit_HardcodedJSON_WarnsOnError(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		// Junior developer hardcoding error response
		ctx.JSON(400, gin.H{"error": "bad request"})
	})

	w := doGet(r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].Message, "non-standard response format")
}

func TestAudit_OldErrorFormat_Warns(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		// Old format that someone copy-pasted
		ctx.JSON(404, gin.H{"status": "error", "message": "not found"})
	})

	w := doGet(r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, 1, logs.Len(), "old error format should trigger warning")
}

func TestAudit_PartialSuccess_Warns(t *testing.T) {
	logger, logs := newObservedLogger()

	r := auditRouter(logger, func(ctx *gin.Context) {
		// Has "status" but missing "data"
		ctx.JSON(200, gin.H{"status": "success", "result": "oops"})
	})

	w := doGet(r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, logs.Len())
}
