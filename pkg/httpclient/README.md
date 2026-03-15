# httpclient — Secure HTTP Client for Service-to-Service Calls

## Why

Prevents juniors from creating raw `http.Client{}` with no timeout, no auth, no TLS, no logging.
Every external service call goes through this base client.

## Security Built-in

| Protection | How |
|-----------|-----|
| TLS 1.2+ minimum | `tls.Config{MinVersion: tls.VersionTLS12}` |
| Request timeout | Default 10s, configurable |
| Response body limit | 10MB max read (prevents memory exhaustion) |
| Auth header on redirect | Stripped on cross-origin redirect |
| Redirect limit | Max 3 redirects |
| Structured logging | Every request logged with method, URL, status, latency |
| Retry with backoff | Only on 502/503/504, configurable |
| Context propagation | Always respects `ctx` cancellation/deadline |

## Usage — Creating a Service Client

### 1. Define service-specific client (embed `httpclient.Client`)

```go
// modules/payment/client.go
package payment

import (
    "context"

    "go.uber.org/zap"

    "github.com/ntthienan0507-web/go-api-template/pkg/httpclient"
)

// Client wraps the base HTTP client for the Payment service.
type Client struct {
    *httpclient.Client
}

// NewClient creates a payment service client.
func NewClient(cfg httpclient.Config, logger *zap.Logger) *Client {
    return &Client{Client: httpclient.New(cfg, logger)}
}

// Charge calls POST /api/charges on the payment service.
func (c *Client) Charge(ctx context.Context, req ChargeRequest) (*ChargeResponse, error) {
    resp, err := c.Post(ctx, "/api/charges", req)
    if err != nil {
        return nil, err
    }

    var result ChargeResponse
    if err := httpclient.DecodeJSON(resp, &result); err != nil {
        return nil, ErrPaymentFailed.WithDetail(err.Error())
    }
    return &result, nil
}

// GetTransaction calls GET /api/transactions/:id
func (c *Client) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
    resp, err := c.Get(ctx, "/api/transactions/"+id)
    if err != nil {
        return nil, err
    }

    var result Transaction
    if err := httpclient.DecodeJSON(resp, &result); err != nil {
        return nil, ErrTransactionNotFound.WithDetail(err.Error())
    }
    return &result, nil
}
```

### 2. Wire in app.go

```go
// in registerModules() or New()
paymentClient := payment.NewClient(httpclient.Config{
    BaseURL:    cfg.PaymentServiceURL,     // from env
    APIKey:     cfg.PaymentServiceAPIKey,   // from env
    Timeout:    5 * time.Second,
    MaxRetries: 2,
}, a.logger)

orderSvc := order.NewService(orderRepo, paymentClient, a.logger)
```

### 3. Use in service

```go
// modules/order/service.go
func (s *Service) Checkout(ctx context.Context, orderID uuid.UUID) error {
    order, err := s.repo.GetByID(ctx, orderID)
    if err != nil {
        return ErrOrderNotFound
    }

    // Call payment service — all security handled by base client
    charge, err := s.paymentClient.Charge(ctx, payment.ChargeRequest{
        Amount:   order.Total,
        Currency: "USD",
        OrderID:  order.ID.String(),
    })
    if err != nil {
        return err // already an AppError from payment.Client
    }

    return s.repo.UpdateStatus(ctx, orderID, "paid", charge.TransactionID)
}
```

## API

| Method | Description |
|--------|-------------|
| `httpclient.New(cfg, logger)` | Create base client with secure defaults |
| `c.Get(ctx, path)` | GET request |
| `c.Post(ctx, path, body)` | POST with JSON body |
| `c.Put(ctx, path, body)` | PUT with JSON body |
| `c.Patch(ctx, path, body)` | PATCH with JSON body |
| `c.Delete(ctx, path)` | DELETE request |
| `c.Do(ctx, req)` | Raw request (escape hatch) |
| `httpclient.DecodeJSON[T](resp, &dest)` | Decode response, error on 4xx/5xx |

## Config

```go
httpclient.Config{
    BaseURL:            "https://api.payment.internal",
    Timeout:            10 * time.Second,  // per-request
    APIKey:             "sk_live_xxx",      // Bearer token
    Headers:            map[string]string{"X-Tenant-ID": "t1"},
    MaxRetries:         2,                  // retry on 502/503/504
    RetryDelay:         500 * time.Millisecond,
    InsecureSkipVerify: false,              // NEVER true in prod
}
```

## Rules

1. **NEVER** create raw `http.Client{}` — always use `httpclient.New()`
2. **NEVER** set `InsecureSkipVerify: true` in production
3. Each external service gets its own client struct that **embeds** `*httpclient.Client`
4. Service-specific methods (e.g. `Charge()`, `GetUser()`) live in the module, not here
5. Return `*apperror.AppError` from service client methods for consistent error handling
