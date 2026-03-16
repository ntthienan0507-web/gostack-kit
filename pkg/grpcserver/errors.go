package grpcserver

import (
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// StatusFromError converts an error to a gRPC status.
// Maps AppError HTTP codes to gRPC status codes and preserves the message key.
// When sanitize is true, Detail is stripped from the response (use in production).
func StatusFromError(err error, sanitize bool) error {
	if err == nil {
		return nil
	}

	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		code := httpToGRPC(appErr.Code)
		msg := appErr.Message
		if !sanitize && appErr.Detail != "" {
			msg += ": " + appErr.Detail
		}
		return status.Error(code, msg)
	}

	return status.Error(codes.Internal, "internal error")
}

func httpToGRPC(httpCode int) codes.Code {
	switch httpCode {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusUnprocessableEntity:
		return codes.FailedPrecondition
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	default:
		return codes.Internal
	}
}
