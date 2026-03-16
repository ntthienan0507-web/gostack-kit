package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

func TestAbort_StopsChain(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(ctx *gin.Context) {
		Abort(ctx, apperror.ErrTokenMissing)
	}, func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{"should": "not appear"})
	})

	w := performRequest(func(ctx *gin.Context) {
		Abort(ctx, apperror.ErrTokenMissing)
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body apperror.AppError
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "common.token_missing", body.Message)
}

func TestError_WritesJSON(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		Error(ctx, apperror.ErrRecordNotFound)
	})

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body apperror.AppError
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "common.record_not_found", body.Message)
	assert.Equal(t, "Record not found", body.Detail)
}

func TestHandleError_PgxNoRows(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		HandleError(ctx, pgx.ErrNoRows)
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleError_UniqueViolation(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		HandleError(ctx, &pgconn.PgError{Code: "23505"})
	})

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandleError_GenericError(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		HandleError(ctx, errors.New("random"))
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestError_Sanitized_ReleaseMode(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(gin.TestMode)

	w := performRequest(func(ctx *gin.Context) {
		Error(ctx, apperror.New(500, "common.internal_error", "pq: relation \"users\" does not exist"))
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "common.internal_error", body["error_message"])
	assert.Empty(t, body["error_detail"], "detail should be stripped in release mode")
}

func TestError_NotSanitized_DebugMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := performRequest(func(ctx *gin.Context) {
		Error(ctx, apperror.New(400, "user.invalid_id", "uuid: invalid format"))
	})

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "uuid: invalid format", body["error_detail"])
}
