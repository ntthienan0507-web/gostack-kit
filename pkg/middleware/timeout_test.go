package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTimeout_SetsDeadline(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(2 * time.Second))
	r.GET("/test", func(c *gin.Context) {
		deadline, ok := c.Request.Context().Deadline()
		assert.True(t, ok, "context should have deadline")
		assert.WithinDuration(t, time.Now().Add(2*time.Second), deadline, 100*time.Millisecond)
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTimeout_ContextCancelledAfterDeadline(t *testing.T) {
	r := gin.New()
	r.Use(Timeout(50 * time.Millisecond))
	r.GET("/slow", func(c *gin.Context) {
		select {
		case <-time.After(200 * time.Millisecond):
			c.Status(200)
		case <-c.Request.Context().Done():
			c.Status(504)
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/slow", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 504, w.Code)
}

func TestLongRunning_5MinDeadline(t *testing.T) {
	r := gin.New()
	r.Use(LongRunning())
	r.GET("/test", func(c *gin.Context) {
		deadline, ok := c.Request.Context().Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(5*time.Minute), deadline, time.Second)
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
