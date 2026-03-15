package icewarp

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// Sentinel errors for IceWarp operations.
var (
	ErrRequestFailed  = apperror.New(http.StatusBadGateway, "icewarp.request_failed", "IceWarp API request failed")
	ErrAuthFailed     = apperror.New(http.StatusBadGateway, "icewarp.auth_failed", "IceWarp authentication failed")
	ErrRateLimited    = apperror.New(http.StatusTooManyRequests, "icewarp.rate_limited", "IceWarp rate limit exceeded")
	ErrDeliveryFailed = apperror.New(http.StatusBadGateway, "icewarp.delivery_failed", "Email delivery failed via IceWarp")
	ErrMailboxError   = apperror.New(http.StatusBadGateway, "icewarp.mailbox_error", "IceWarp mailbox error")
	ErrAccountExists  = apperror.New(http.StatusConflict, "icewarp.account_exists", "IceWarp account already exists")
)

// apiErrorResponse represents the XML error response from the IceWarp API.
type apiErrorResponse struct {
	XMLName xml.Name `xml:"ErrorResponse"`
	Code    int      `xml:"Code"`
	Message string   `xml:"Message"`
}

// errorDecoder implements httpclient.ErrorDecoder for IceWarp.
type errorDecoder struct{}

// DecodeError decodes an IceWarp XML error response into a domain error.
func (d *errorDecoder) DecodeError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return ErrRequestFailed.WithDetail("failed to read error response")
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrAuthFailed
	}

	var apiResp apiErrorResponse
	if err := xml.Unmarshal(body, &apiResp); err == nil && apiResp.Code != 0 {
		return d.mapErrorCode(apiResp.Code, apiResp.Message)
	}

	return ErrRequestFailed.WithDetail(fmt.Sprintf("icewarp returned %d", resp.StatusCode))
}

// mapErrorCode maps IceWarp error codes to domain errors.
// Code ranges:
//   - 2000-2999: mailbox errors
//   - 3000-3999: delivery errors
func (d *errorDecoder) mapErrorCode(code int, message string) error {
	switch {
	case code >= 2000 && code < 3000:
		return ErrMailboxError.WithDetail(fmt.Sprintf("icewarp mailbox error %d: %s", code, message))
	case code >= 3000 && code < 4000:
		return ErrDeliveryFailed.WithDetail(fmt.Sprintf("icewarp delivery error %d: %s", code, message))
	default:
		return ErrRequestFailed.WithDetail(fmt.Sprintf("icewarp error %d: %s", code, message))
	}
}
