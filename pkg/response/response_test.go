package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func performRequest(handler gin.HandlerFunc) *httptest.ResponseRecorder {
	r := gin.New()
	r.GET("/test", handler)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	return w
}

func TestOK(t *testing.T) {
	type data struct {
		Name string `json:"name"`
	}
	w := performRequest(func(ctx *gin.Context) {
		OK(ctx, data{Name: "test"})
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var body Response[data]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body.Status)
	assert.Equal(t, "test", body.Data.Name)
}

func TestOKList(t *testing.T) {
	items := []string{"a", "b", "c"}
	w := performRequest(func(ctx *gin.Context) {
		OKList(ctx, items, 10)
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var body Response[ListData[string]]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body.Status)
	assert.Equal(t, []string{"a", "b", "c"}, body.Data.Items)
	assert.Equal(t, int64(10), body.Data.Total)
}

func TestCreated(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		Created(ctx, map[string]string{"id": "123"})
	})

	assert.Equal(t, http.StatusCreated, w.Code)

	var body Response[map[string]string]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body.Status)
	assert.Equal(t, "123", body.Data["id"])
}

func TestNoContent(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		NoContent(ctx)
	})

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestOKList_EmptySlice(t *testing.T) {
	w := performRequest(func(ctx *gin.Context) {
		OKList(ctx, []string{}, 0)
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var body Response[ListData[string]]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Empty(t, body.Data.Items)
	assert.Equal(t, int64(0), body.Data.Total)
}

func TestOK_NestedStruct(t *testing.T) {
	type inner struct {
		Value int `json:"value"`
	}
	type outer struct {
		Inner inner `json:"inner"`
	}

	w := performRequest(func(ctx *gin.Context) {
		OK(ctx, outer{Inner: inner{Value: 42}})
	})

	assert.Equal(t, http.StatusOK, w.Code)

	var body Response[outer]
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, 42, body.Data.Inner.Value)
}
