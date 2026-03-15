package apperror

import "net/http"

// Common error codes — shared across all modules.
// Namespace: "common.*"
var (
	ErrBadRequest         = New(http.StatusBadRequest, "common.bad_request", "Invalid request")
	ErrInvalidParams      = New(http.StatusBadRequest, "common.invalid_params", "Invalid request parameters")
	ErrRequiredFieldMissing = New(http.StatusBadRequest, "common.required_field_missing", "A required field is missing")
	ErrValidationFailed   = New(http.StatusBadRequest, "common.validation_failed", "Request validation failed")

	ErrUnauthorized       = New(http.StatusUnauthorized, "common.unauthorized", "Authentication required")
	ErrTokenMissing       = New(http.StatusUnauthorized, "common.token_missing", "Missing bearer token")
	ErrTokenInvalid       = New(http.StatusUnauthorized, "common.token_invalid", "Invalid or expired token")

	ErrForbidden          = New(http.StatusForbidden, "common.forbidden", "Insufficient permissions")

	ErrRecordNotFound        = New(http.StatusNotFound, "common.record_not_found", "Record not found")
	ErrRouteNotFound         = New(http.StatusNotFound, "common.route_not_found", "Route not found")

	ErrRecordAlreadyExists   = New(http.StatusConflict, "common.record_already_exists", "Record already exists")
	ErrStaleVersion          = New(http.StatusConflict, "common.stale_version", "The record was modified by another request. Please retry with the latest version.")
	ErrRelatedRecordNotFound = New(http.StatusUnprocessableEntity, "common.related_record_not_found", "Related record not found")

	ErrRateLimited           = New(http.StatusTooManyRequests, "common.rate_limited", "Too many requests, please try again later")

	ErrInternalError         = New(http.StatusInternalServerError, "common.internal_error", "An internal error occurred")
)
