package user

import (
	"net/http"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// User module error codes.
// Namespace: "user.*"
var (
	ErrUserNotFound   = apperror.New(http.StatusNotFound, "user.not_found", "User with the given ID does not exist")
	ErrUserExists     = apperror.New(http.StatusConflict, "user.already_exists", "A user with this username or email already exists")
	ErrInvalidUserID  = apperror.New(http.StatusBadRequest, "user.invalid_id", "Invalid user ID format")
	ErrPasswordTooWeak = apperror.New(http.StatusBadRequest, "user.password_too_weak", "Password does not meet strength requirements")
)
