package sendgrid

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/go-api-template/pkg/httpclient"
	"github.com/ntthienan0507-web/go-api-template/pkg/retry"
)

const defaultBaseURL = "https://api.sendgrid.com"

// Config holds SendGrid client configuration.
type Config struct {
	BaseURL   string // defaults to https://api.sendgrid.com
	APIKey    string
	FromEmail string
	FromName  string
}

// Client wraps httpclient.ServiceClient for SendGrid operations.
type Client struct {
	sc       *httpclient.ServiceClient
	fromAddr Address
}

// New creates a new SendGrid client.
func New(cfg Config, logger *zap.Logger) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	base := httpclient.New(httpclient.Config{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Retry: &retry.DefaultConfig,
	}, logger)

	sc := httpclient.NewServiceClient(base, httpclient.ServiceConfig{
		ErrorDecoder: &errorDecoder{},
	})

	return &Client{
		sc: sc,
		fromAddr: Address{
			Email: cfg.FromEmail,
			Name:  cfg.FromName,
		},
	}
}

// Send sends an email via the SendGrid v3 API.
// If req.From is empty, it uses the default from address configured in Config.
func (c *Client) Send(ctx context.Context, req SendRequest) error {
	if req.From.Email == "" {
		req.From = c.fromAddr
	}

	return httpclient.DoVoid(ctx, c.sc, http.MethodPost, "/v3/mail/send", req)
}
