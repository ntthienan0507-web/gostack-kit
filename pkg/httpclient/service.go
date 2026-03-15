package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

// ContentType represents the content type for requests and responses.
type ContentType int

const (
	// ContentJSON is the default JSON content type.
	ContentJSON ContentType = iota
	// ContentXML is the XML content type.
	ContentXML
)

// ErrorDecoder decodes an HTTP error response into a Go error.
// Implementations should inspect the response body and status code
// and return a domain-specific error.
type ErrorDecoder interface {
	DecodeError(resp *http.Response) error
}

// ServiceConfig holds configuration for a ServiceClient.
type ServiceConfig struct {
	// ErrorDecoder decodes upstream error responses into domain errors.
	ErrorDecoder ErrorDecoder

	// ContentType sets the request/response content type (default: ContentJSON).
	ContentType ContentType
}

// ServiceClient wraps Client with service-specific error decoding and content type handling.
// Use this to build external service integrations with structured error handling.
type ServiceClient struct {
	*Client
	errorDecoder ErrorDecoder
	contentType  ContentType
}

// NewServiceClient creates a ServiceClient from a base Client and service-specific config.
func NewServiceClient(client *Client, cfg ServiceConfig) *ServiceClient {
	return &ServiceClient{
		Client:       client,
		errorDecoder: cfg.ErrorDecoder,
		contentType:  cfg.ContentType,
	}
}

// Do executes a request with the service's content type and error decoding.
// It returns the deserialized response of type T.
func Do[T any](ctx context.Context, sc *ServiceClient, method, path string, body any) (*T, error) {
	resp, err := sc.doRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, sc.decodeError(resp)
	}

	var result T
	if err := sc.decode(resp, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// DoVoid executes a request that expects no response body (e.g. 202 Accepted).
func DoVoid(ctx context.Context, sc *ServiceClient, method, path string, body any) error {
	resp, err := sc.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return sc.decodeError(resp)
	}

	// Drain body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// doRequest builds and executes the HTTP request with the appropriate content type.
func (sc *ServiceClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		switch sc.contentType {
		case ContentXML:
			if err := xml.NewEncoder(&buf).Encode(body); err != nil {
				return nil, fmt.Errorf("encode xml body: %w", err)
			}
		default:
			if err := json.NewEncoder(&buf).Encode(body); err != nil {
				return nil, fmt.Errorf("encode json body: %w", err)
			}
		}
	}

	req, err := http.NewRequest(method, sc.url(path), &buf)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	switch sc.contentType {
	case ContentXML:
		req.Header.Set("Content-Type", "application/xml")
		req.Header.Set("Accept", "application/xml")
	default:
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
	}

	return sc.Client.Do(ctx, req)
}

// decodeError uses the service's ErrorDecoder or falls back to a generic error.
func (sc *ServiceClient) decodeError(resp *http.Response) error {
	if sc.errorDecoder != nil {
		if err := sc.errorDecoder.DecodeError(resp); err != nil {
			return err
		}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	sc.Client.logger.Warn("upstream error",
		zap.Int("status", resp.StatusCode),
		zap.String("body", string(body)),
	)
	return fmt.Errorf("upstream returned %d: %s", resp.StatusCode, truncateBody(body, 200))
}

// decode deserializes the response body based on content type.
func (sc *ServiceClient) decode(resp *http.Response, dest any) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	switch sc.contentType {
	case ContentXML:
		return xml.Unmarshal(body, dest)
	default:
		return json.Unmarshal(body, dest)
	}
}
