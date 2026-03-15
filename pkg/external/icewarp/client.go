package icewarp

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/httpclient"
	"github.com/ntthienan0507-web/gostack-kit/pkg/retry"
)

// Config holds IceWarp client configuration.
type Config struct {
	URL      string // IceWarp API URL
	Username string
	Password string
	From     string // default sender email address
}

// Client wraps httpclient.ServiceClient for IceWarp operations.
type Client struct {
	sc   *httpclient.ServiceClient
	from string
}

// New creates a new IceWarp client.
func New(cfg Config, logger *zap.Logger) *Client {
	base := httpclient.New(httpclient.Config{
		BaseURL: cfg.URL,
		Headers: map[string]string{
			"Content-Type": "application/xml",
		},
		TokenSource: httpclient.NewStaticToken(cfg.Password),
		Retry:       &retry.DefaultConfig,
	}, logger)

	sc := httpclient.NewServiceClient(base, httpclient.ServiceConfig{
		ContentType:  httpclient.ContentXML,
		ErrorDecoder: &errorDecoder{},
	})

	return &Client{
		sc:   sc,
		from: cfg.From,
	}
}

// Send sends an email via the IceWarp API.
func (c *Client) Send(ctx context.Context, req SendRequest) (*SendResponse, error) {
	if req.From == "" {
		req.From = c.from
	}

	resp, err := httpclient.Do[SendResponse](ctx, c.sc, http.MethodPost, "/api/mail/send", req)
	if err != nil {
		return nil, fmt.Errorf("icewarp send: %w", err)
	}
	return resp, nil
}

// GetAccount retrieves account information for the given email.
func (c *Client) GetAccount(ctx context.Context, email string) (*AccountInfo, error) {
	resp, err := httpclient.Do[AccountInfo](ctx, c.sc, http.MethodGet, fmt.Sprintf("/api/accounts/%s", email), nil)
	if err != nil {
		return nil, fmt.Errorf("icewarp get account: %w", err)
	}
	return resp, nil
}

// CreateAccount creates a new IceWarp account.
func (c *Client) CreateAccount(ctx context.Context, req CreateAccountRequest) (*CreateAccountResponse, error) {
	resp, err := httpclient.Do[CreateAccountResponse](ctx, c.sc, http.MethodPost, "/api/accounts", req)
	if err != nil {
		return nil, fmt.Errorf("icewarp create account: %w", err)
	}
	return resp, nil
}

// DeleteAccount deletes an IceWarp account by email.
func (c *Client) DeleteAccount(ctx context.Context, email string) error {
	if err := httpclient.DoVoid(ctx, c.sc, http.MethodDelete, fmt.Sprintf("/api/accounts/%s", email), nil); err != nil {
		return fmt.Errorf("icewarp delete account: %w", err)
	}
	return nil
}
