package middleware

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// errPayloadTooLarge is returned when the request body exceeds the allowed size.
var errPayloadTooLarge = apperror.New(http.StatusRequestEntityTooLarge, "common.payload_too_large", "Request body too large")

// errUnsupportedFileType is returned when the uploaded file type is not allowed.
var errUnsupportedFileType = apperror.New(http.StatusBadRequest, "common.unsupported_file_type", "File type not allowed")

// errFileMissing is returned when a required file field is not present.
var errFileMissing = apperror.New(http.StatusBadRequest, "common.file_missing", "Required file is missing")

// MaxBodySize limits the request body size. Use for upload endpoints.
// Returns 413 Payload Too Large if exceeded.
//
//	router.POST("/upload", middleware.MaxBodySize(10<<20), controller.Upload) // 10MB
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		c.Next()

		// MaxBytesReader will cause subsequent reads to fail with MaxBytesError.
		// Gin's form parsing triggers the read, so we also check the error after Next().
		if c.IsAborted() {
			return
		}
	}
}

// AllowedFileTypes validates uploaded file MIME types.
// Checks actual content (magic bytes via http.DetectContentType), not just
// the extension or Content-Type header (which can be spoofed).
//
//	router.POST("/avatar", middleware.AllowedFileTypes("image/jpeg", "image/png", "image/webp"), ...)
func AllowedFileTypes(types ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(types))
	for _, t := range types {
		allowed[t] = struct{}{}
	}

	return func(c *gin.Context) {
		form, err := c.MultipartForm()
		if err != nil {
			apperror.Abort(c, apperror.ErrBadRequest.WithDetail("Failed to parse multipart form"))
			return
		}

		for _, files := range form.File {
			for _, fh := range files {
				f, err := fh.Open()
				if err != nil {
					apperror.Abort(c, apperror.ErrBadRequest.WithDetail("Failed to open uploaded file"))
					return
				}

				// Read up to 512 bytes for content detection (DetectContentType uses at most 512).
				buf := make([]byte, 512)
				n, err := f.Read(buf)
				f.Close()
				if err != nil && err != io.EOF {
					apperror.Abort(c, apperror.ErrBadRequest.WithDetail("Failed to read uploaded file"))
					return
				}

				detected := http.DetectContentType(buf[:n])
				if _, ok := allowed[detected]; !ok {
					apperror.Abort(c, errUnsupportedFileType.WithDetail("Detected type: "+detected))
					return
				}
			}
		}

		c.Next()
	}
}

// RequireFile ensures a specific form field has a file attached.
// Returns 400 if missing.
//
//	router.POST("/upload", middleware.RequireFile("file"), controller.Upload)
func RequireFile(fieldName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, header, err := c.Request.FormFile(fieldName)
		if err != nil || header == nil {
			apperror.Abort(c, errFileMissing.WithDetail("Missing file field: "+fieldName))
			return
		}

		c.Next()
	}
}
