package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/pkg/circuitbreaker"
	"github.com/ntthienan0507-web/go-api-template/pkg/retry"
)

// Config holds HTTP client settings for a single external service.
type Config struct {
	BaseURL string        // e.g. "https://api.payment.internal"
	Timeout time.Duration // per-request timeout (default 10s)
	Headers map[string]string // extra default headers

	// Auth — pick ONE:
	//   TokenSource: dynamic tokens (OAuth2 client_credentials, token forwarding, etc.)
	//   APIKey:      static API key (backward compat — internally wraps as StaticToken)
	TokenSource TokenSource // preferred — dynamic token management
	APIKey      string      // simple static key (ignored if TokenSource is set)

	// TLS
	InsecureSkipVerify bool // ONLY for dev/test — never in production

	// Network
	SourceIP string // bind outbound requests to this local IP (for IP whitelist scenarios)

	// Retry (uses pkg/retry with exponential backoff + jitter)
	Retry *retry.Config // nil = no retry

	// CircuitBreaker — nil = disabled. Wraps all outbound calls.
	CircuitBreaker *circuitbreaker.CircuitBreaker
}

// Client is the base HTTP client with security defaults.
// Embed this in service-specific clients — do NOT create raw http.Client.
type Client struct {
	http           *http.Client
	baseURL        string
	tokenSource    TokenSource // nil = no auth
	headers        map[string]string
	logger         *zap.Logger
	retry          retry.Config
	circuitBreaker *circuitbreaker.CircuitBreaker // nil = disabled
}

// New creates a Client with secure defaults.
func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	retryCfg := retry.Config{} // no retry by default
	if cfg.Retry != nil {
		retryCfg = *cfg.Retry
	}

	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Bind to a specific source IP for outbound requests.
	// Required when the 3rd party whitelists your server IP and the server has multiple IPs.
	if cfg.SourceIP != "" {
		dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(cfg.SourceIP)}
		logger.Info("httpclient binding to source IP", zap.String("ip", cfg.SourceIP))
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // configurable for dev
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext:         dialer.DialContext,
	}

	// Resolve token source: explicit TokenSource > APIKey > none
	var ts TokenSource
	switch {
	case cfg.TokenSource != nil:
		ts = cfg.TokenSource
	case cfg.APIKey != "":
		ts = NewStaticToken(cfg.APIKey)
	}

	return &Client{
		http: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
			// Don't follow redirects automatically — prevent open redirect attacks
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("stopped after 3 redirects")
				}
				// Strip auth header on cross-origin redirect
				if len(via) > 0 && req.URL.Host != via[0].URL.Host {
					req.Header.Del("Authorization")
				}
				return nil
			},
		},
		baseURL:        strings.TrimRight(cfg.BaseURL, "/"),
		tokenSource:    ts,
		headers:        cfg.Headers,
		logger:         logger,
		retry:          retryCfg,
		circuitBreaker: cfg.CircuitBreaker,
	}
}

// Ping verifies connectivity to the service's base URL.
// Call this at startup to fail fast if the service is unreachable (e.g. IP not whitelisted).
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequest(http.MethodHead, c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("ping build request: %w", err)
	}
	req = req.WithContext(ctx)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ping %s failed (check IP whitelist / network): %w", c.baseURL, err)
	}
	resp.Body.Close()

	c.logger.Info("ping ok", zap.String("url", c.baseURL), zap.Int("status", resp.StatusCode))
	return nil
}

// Do executes a raw *http.Request with auth headers, logging, circuit breaker, and retry.
// Prefer Get/Post/Put/Delete helpers instead of calling this directly.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	c.setHeaders(ctx, req)

	// If circuit breaker is configured, wrap the entire call (including retries).
	if c.circuitBreaker != nil {
		return circuitbreaker.Execute(c.circuitBreaker, func() (*http.Response, error) {
			return c.doWithRetry(ctx, req)
		})
	}

	return c.doWithRetry(ctx, req)
}

// doWithRetry handles single-attempt or retry logic.
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	// No retry configured — single attempt
	if c.retry.MaxRetries == 0 {
		return c.doOnce(req)
	}

	// Retry with exponential backoff
	return retry.DoWithResult(ctx, c.retry, func() (*http.Response, error) {
		resp, err := c.doOnce(req)
		if err != nil {
			return nil, err // network errors → retryable by default
		}

		// Map 5xx/429 to retry.HTTPError so retry.IsRetryable picks them up
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			return nil, retry.NewHTTPError(resp.StatusCode, fmt.Sprintf("upstream %d", resp.StatusCode))
		}

		return resp, nil
	})
}

// --- Convenience methods ---

// Get performs a GET request to baseURL + path.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.url(path), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	return c.Do(ctx, req)
}

// Post performs a POST request with JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doJSON(ctx, http.MethodPost, path, body)
}

// Put performs a PUT request with JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doJSON(ctx, http.MethodPut, path, body)
}

// Patch performs a PATCH request with JSON body.
func (c *Client) Patch(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doJSON(ctx, http.MethodPatch, path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, c.url(path), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	return c.Do(ctx, req)
}

// --- Response helpers ---

// DecodeJSON reads and decodes a JSON response body into dest.
// Always closes the body. Returns error on non-2xx status.
func DecodeJSON[T any](resp *http.Response, dest *T) error {
	defer resp.Body.Close()

	// Limit body read to 10MB to prevent memory exhaustion
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("upstream returned %d: %s", resp.StatusCode, truncateBody(body, 200))
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// --- Internal ---

func (c *Client) setHeaders(ctx context.Context, req *http.Request) {
	if req.Header.Get("Content-Type") == "" && req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	// Resolve token dynamically — supports static, OAuth2, forwarding, etc.
	if c.tokenSource != nil && req.Header.Get("Authorization") == "" {
		if token, err := c.tokenSource.Token(ctx); err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else if err != nil {
			c.logger.Warn("token source failed", zap.Error(err))
		}
	}
	for k, v := range c.headers {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
}

func (c *Client) doOnce(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := c.http.Do(req)
	latency := time.Since(start)

	if err != nil {
		c.logger.Error("http request failed",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		return nil, err
	}

	c.logger.Debug("http request",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.Int("status", resp.StatusCode),
		zap.Duration("latency", latency),
	)

	return resp, nil
}

func (c *Client) url(path string) string {
	if path == "" {
		return c.baseURL
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode body: %w", err)
		}
	}

	req, err := http.NewRequest(method, c.url(path), &buf)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.Do(ctx, req)
}

func truncateBody(body []byte, max int) string {
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}
