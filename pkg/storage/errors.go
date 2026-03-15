package storage

import (
	"net/http"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// Storage error codes.
// Namespace: "storage.*"
var (
	ErrObjectNotFound = apperror.New(http.StatusNotFound, "storage.object_not_found", "Object not found")
	ErrUploadFailed   = apperror.New(http.StatusBadGateway, "storage.upload_failed", "Failed to upload object")
	ErrDownloadFailed = apperror.New(http.StatusBadGateway, "storage.download_failed", "Failed to download object")
	ErrPresignFailed  = apperror.New(http.StatusBadGateway, "storage.presign_failed", "Failed to generate presigned URL")
)
