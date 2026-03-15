package stripe

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/pkg/httpclient"
	"github.com/ntthienan0507-web/go-api-template/pkg/retry"
)

const defaultBaseURL = "https://api.stripe.com"

// Config holds Stripe client configuration.
type Config struct {
	BaseURL   string // defaults to https://api.stripe.com
	SecretKey string
}

// Client wraps httpclient.ServiceClient for Stripe operations.
type Client struct {
	sc *httpclient.ServiceClient
}

// New creates a new Stripe client.
func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	base := httpclient.New(httpclient.Config{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.SecretKey,
		Retry:   &retry.DefaultConfig,
	}, logger)

	sc := httpclient.NewServiceClient(base, httpclient.ServiceConfig{
		ErrorDecoder: &errorDecoder{},
	})

	return &Client{sc: sc}
}

// Charge creates a new charge.
func (c *Client) Charge(ctx context.Context, req ChargeRequest) (*ChargeResponse, error) {
	resp, err := httpclient.Do[ChargeResponse](ctx, c.sc, http.MethodPost, "/v1/charges", req)
	if err != nil {
		return nil, fmt.Errorf("stripe charge: %w", err)
	}
	return resp, nil
}

// Refund creates a refund for an existing charge.
func (c *Client) Refund(ctx context.Context, req RefundRequest) (*RefundResponse, error) {
	resp, err := httpclient.Do[RefundResponse](ctx, c.sc, http.MethodPost, "/v1/refunds", req)
	if err != nil {
		return nil, fmt.Errorf("stripe refund: %w", err)
	}
	return resp, nil
}
