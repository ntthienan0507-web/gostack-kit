package middleware

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- MaxBodySize ---

func TestMaxBodySize_AllowsSmallPayload(t *testing.T) {
	r := gin.New()
	r.POST("/upload", MaxBodySize(1<<20), func(c *gin.Context) { // 1MB
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.String(http.StatusOK, "received %d bytes", len(body))
	})

	payload := strings.NewReader("hello")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/upload", payload)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "received 5 bytes")
}

func TestMaxBodySize_BlocksLargePayload(t *testing.T) {
	r := gin.New()
	r.POST("/upload", MaxBodySize(10), func(c *gin.Context) { // 10 bytes max
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusOK)
	})

	// Send more than 10 bytes.
	payload := strings.NewReader("this is way more than ten bytes of data")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/upload", payload)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// --- AllowedFileTypes ---

func createMultipartRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, err = part.Write(content)
	if err != nil {
		t.Fatal(err)
	}
	writer.Close()

	req, _ := http.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestAllowedFileTypes_AcceptsValidType(t *testing.T) {
	r := gin.New()
	r.POST("/upload", AllowedFileTypes("image/png"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Create a real PNG image so DetectContentType recognizes it.
	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	err := png.Encode(&pngBuf, img)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	req := createMultipartRequest(t, "file", "test.png", pngBuf.Bytes())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAllowedFileTypes_RejectsInvalidType(t *testing.T) {
	r := gin.New()
	r.POST("/upload", AllowedFileTypes("image/png"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Send plain text content — DetectContentType will detect "text/plain".
	content := []byte("this is not an image, just plain text content for testing")

	w := httptest.NewRecorder()
	req := createMultipartRequest(t, "file", "fake.png", content)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported_file_type")
}

// --- RequireFile ---

func TestRequireFile_ReturnsOKWhenPresent(t *testing.T) {
	r := gin.New()
	r.POST("/upload", RequireFile("file"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := createMultipartRequest(t, "file", "test.txt", []byte("hello"))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireFile_Returns400WhenMissing(t *testing.T) {
	r := gin.New()
	r.POST("/upload", RequireFile("file"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Send request with no file.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/upload", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file_missing")
}

func TestRequireFile_Returns400WhenWrongFieldName(t *testing.T) {
	r := gin.New()
	r.POST("/upload", RequireFile("avatar"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Upload file under "file" field, but middleware expects "avatar".
	w := httptest.NewRecorder()
	req := createMultipartRequest(t, "file", "test.txt", []byte("hello"))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file_missing")
}
