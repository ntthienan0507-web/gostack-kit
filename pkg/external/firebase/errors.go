package firebase

import (
	"net/http"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// Sentinel errors for Firebase operations.
var (
	ErrInitFailed         = apperror.New(http.StatusBadGateway, "firebase.init_failed", "Failed to initialize Firebase")
	ErrPushFailed         = apperror.New(http.StatusBadGateway, "firebase.push_failed", "Failed to send push notification")
	ErrInvalidToken       = apperror.New(http.StatusBadRequest, "firebase.invalid_token", "Invalid FCM device token")
	ErrTokenNotRegistered = apperror.New(http.StatusGone, "firebase.token_not_registered", "FCM token is no longer registered")
	ErrVerifyFailed       = apperror.New(http.StatusUnauthorized, "firebase.verify_failed", "Failed to verify Firebase ID token")
)
