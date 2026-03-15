package stripe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// Sentinel errors for Stripe operations.
var (
	ErrChargeFailed   = apperror.New(http.StatusBadGateway, "stripe.charge_failed", "Failed to create charge via Stripe")
	ErrCardDeclined   = apperror.New(http.StatusUnprocessableEntity, "stripe.card_declined", "Card was declined")
	ErrInvalidRequest = apperror.New(http.StatusBadRequest, "stripe.invalid_request", "Invalid request to Stripe")
	ErrRateLimited    = apperror.New(http.StatusTooManyRequests, "stripe.rate_limited", "Stripe rate limit exceeded")
	ErrAuthFailed     = apperror.New(http.StatusBadGateway, "stripe.auth_failed", "Stripe authentication failed")
)

// apiError represents a single error from the Stripe API.
type apiError struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// apiErrorResponse represents the error envelope from the Stripe API.
type apiErrorResponse struct {
	Error apiError `json:"error"`
}

// errorDecoder implements httpclient.ErrorDecoder for Stripe.
type errorDecoder struct{}

// DecodeError decodes a Stripe error response into a domain error.
func (d *errorDecoder) DecodeError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return ErrChargeFailed.WithDetail("failed to read error response")
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusUnauthorized:
		return ErrAuthFailed
	}

	var apiResp apiErrorResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Error.Type != "" {
		switch apiResp.Error.Type {
		case "card_error":
			return ErrCardDeclined.WithDetail(apiResp.Error.Message)
		case "invalid_request_error":
			return ErrInvalidRequest.WithDetail(apiResp.Error.Message)
		}

		return ErrChargeFailed.WithDetail(fmt.Sprintf("stripe: %s — %s", apiResp.Error.Type, apiResp.Error.Message))
	}

	return ErrChargeFailed.WithDetail(fmt.Sprintf("stripe returned %d", resp.StatusCode))
}
