package sendgrid

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// Sentinel errors for SendGrid operations.
var (
	ErrSendFailed  = apperror.New(http.StatusBadGateway, "sendgrid.send_failed", "Failed to send email via SendGrid")
	ErrRateLimited = apperror.New(http.StatusTooManyRequests, "sendgrid.rate_limited", "SendGrid rate limit exceeded")
	ErrAuthFailed  = apperror.New(http.StatusBadGateway, "sendgrid.auth_failed", "SendGrid authentication failed")
)

// apiError represents a single error from the SendGrid API.
type apiError struct {
	Message string `json:"message"`
	Field   string `json:"field"`
	Help    string `json:"help"`
}

// apiErrorResponse represents the error response from the SendGrid API.
type apiErrorResponse struct {
	Errors []apiError `json:"errors"`
}

// errorDecoder implements httpclient.ErrorDecoder for SendGrid.
type errorDecoder struct{}

// DecodeError decodes a SendGrid error response into a domain error.
func (d *errorDecoder) DecodeError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return ErrSendFailed.WithDetail("failed to read error response")
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrAuthFailed
	}

	var apiResp apiErrorResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && len(apiResp.Errors) > 0 {
		return ErrSendFailed.WithDetail(fmt.Sprintf("sendgrid: %s", apiResp.Errors[0].Message))
	}

	return ErrSendFailed.WithDetail(fmt.Sprintf("sendgrid returned %d", resp.StatusCode))
}
