package grpcserver

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

func TestStatusFromError_Nil(t *testing.T) {
	assert.Nil(t, StatusFromError(nil, false))
}

func TestStatusFromError_AppError(t *testing.T) {
	tests := []struct {
		httpCode int
		grpcCode codes.Code
	}{
		{http.StatusBadRequest, codes.InvalidArgument},
		{http.StatusUnauthorized, codes.Unauthenticated},
		{http.StatusForbidden, codes.PermissionDenied},
		{http.StatusNotFound, codes.NotFound},
		{http.StatusConflict, codes.AlreadyExists},
		{http.StatusTooManyRequests, codes.ResourceExhausted},
		{http.StatusInternalServerError, codes.Internal},
	}

	for _, tt := range tests {
		appErr := apperror.New(tt.httpCode, "test.key", "detail")
		grpcErr := StatusFromError(appErr, false)

		assert.Equal(t, tt.grpcCode, status.Code(grpcErr), "HTTP %d → gRPC %s", tt.httpCode, tt.grpcCode)
	}
}

func TestStatusFromError_GenericError(t *testing.T) {
	err := fmt.Errorf("something broke")
	grpcErr := StatusFromError(err, false)

	assert.Equal(t, codes.Internal, status.Code(grpcErr))
}

func TestStatusFromError_Sanitized(t *testing.T) {
	appErr := apperror.New(http.StatusNotFound, "user.not_found", "SELECT * FROM users WHERE id=$1")

	grpcErr := StatusFromError(appErr, true)
	st, _ := status.FromError(grpcErr)

	assert.Equal(t, codes.NotFound, st.Code())
	assert.Equal(t, "user.not_found", st.Message())
}

func TestStatusFromError_NotSanitized(t *testing.T) {
	appErr := apperror.New(http.StatusNotFound, "user.not_found", "no rows")

	grpcErr := StatusFromError(appErr, false)
	st, _ := status.FromError(grpcErr)

	assert.Equal(t, "user.not_found: no rows", st.Message())
}
