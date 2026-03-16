# AppError — Structured Error Handling

## Response Format

All errors return this JSON structure:

```json
{
  "error_code": 404,
  "error_message": "user.not_found",
  "error_detail": "User with the given ID does not exist"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `error_code` | `int` | HTTP status code |
| `error_message` | `string` | Snake_case key, namespaced by module. Clients use this for i18n translation |
| `error_detail` | `string` | Human-readable description for debugging. **Stripped in release mode** |

## Architecture

```
pkg/apperror/              ← Pure error types (no gin, no HTTP framework)
  error.go                 # AppError, New(), FromError(), Sanitize()
  common.go                # Shared errors: common.* namespace

pkg/response/              ← HTTP error response (gin-specific)
  error.go                 # Error(), Abort(), HandleError()

pkg/grpcserver/            ← gRPC error response
  errors.go                # StatusFromError() — AppError → gRPC status

pkg/middleware/            ← Middleware error handling
  error.go                 # abortWithAppError() — direct abort, no response dependency

modules/<module>/
  errors.go                # Module-specific errors: <module>.* namespace
```

**Rule:** `common.go` is locked — only infrastructure errors live here (auth, validation, DB).
Each module owns its own `errors.go`. No cross-module error imports.

## How to Add Errors for a New Module

### 1. Create `modules/<module>/errors.go`

```go
package order

import (
    "net/http"

    "github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

var (
    ErrOrderNotFound     = apperror.New(http.StatusNotFound, "order.not_found", "Order does not exist")
    ErrOrderAlreadyPaid  = apperror.New(http.StatusConflict, "order.already_paid", "Order has already been paid")
    ErrInvalidOrderID    = apperror.New(http.StatusBadRequest, "order.invalid_id", "Invalid order ID format")
)
```

### 2. Use in HTTP Controller

```go
func (c *Controller) GetByID(ctx *gin.Context) {
    id, err := uuid.Parse(ctx.Param("id"))
    if err != nil {
        response.Error(ctx, ErrInvalidOrderID)
        return
    }

    order, err := c.service.GetByID(ctx.Request.Context(), id)
    if err != nil {
        response.HandleError(ctx, err)
        return
    }

    response.OK(ctx, order)
}
```

### 3. Use in gRPC Handler

```go
func (h *GRPCHandler) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
    order, err := h.service.GetByID(ctx, id)
    if err != nil {
        return nil, grpcserver.StatusFromError(err, isRelease)
    }
    return toProto(order), nil
}
```

### 4. Use in Service (return AppError directly)

```go
func (s *Service) Pay(ctx context.Context, id uuid.UUID) error {
    order, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return ErrOrderNotFound
    }
    if order.Status == "paid" {
        return ErrOrderAlreadyPaid
    }
    return s.repo.UpdateStatus(ctx, id, "paid")
}
```

### 5. Override Detail with Context

```go
return ErrInsufficientStock.WithDetail(
    fmt.Sprintf("Requested %d but only %d available", requested, available),
)
```

## API Reference

### `pkg/apperror` (transport-agnostic)

| Function | Usage |
|----------|-------|
| `apperror.New(code, message, detail)` | Create a new AppError |
| `apperror.FromError(err)` | Convert any error → AppError (pgx, mongo, etc.) |
| `err.WithDetail(detail)` | Copy error with custom detail message |
| `err.Sanitize()` | Copy error with detail stripped (production) |

### `pkg/response` (HTTP/gin)

| Function | Usage |
|----------|-------|
| `response.Error(ctx, appErr)` | Write AppError as JSON |
| `response.Abort(ctx, appErr)` | Write + abort middleware chain |
| `response.HandleError(ctx, err)` | Auto-convert any error → AppError → JSON |

### `pkg/grpcserver` (gRPC)

| Function | Usage |
|----------|-------|
| `grpcserver.StatusFromError(err, sanitize)` | Convert error → gRPC status code |

## Error Flow

```
HTTP Controller                     gRPC Handler
    │                                    │
    │  ← return ErrOrderNotFound ────────┘  (module AppError)
    │  ← return pgx.ErrNoRows ──────────┘  (raw DB error)
    │                                    │
    ├─ response.HandleError(ctx, err)    ├─ grpcserver.StatusFromError(err, sanitize)
    │       │                            │       │
    │       └─ apperror.FromError(err)   │       └─ apperror.FromError(err)
    │              ├─ *AppError → as-is  │              ├─ *AppError → gRPC code
    │              ├─ pgx.ErrNoRows → 404│              ├─ pgx.ErrNoRows → NotFound
    │              └─ unknown → 500      │              └─ unknown → Internal
    │                                    │
    └─ JSON response                     └─ gRPC status
```

## Detail Sanitization

In **release mode**, `error_detail` is stripped from responses to prevent leaking internal info:

```
Debug mode:   {"error_code": 500, "error_message": "common.internal_error", "error_detail": "pq: relation \"users\" does not exist"}
Release mode: {"error_code": 500, "error_message": "common.internal_error"}
```

This applies automatically via `response.Error/Abort/HandleError` (HTTP) and `grpcserver.StatusFromError(err, true)` (gRPC).

## Common Errors (DO NOT add module-specific errors here)

| Key | Code | When to use |
|-----|------|-------------|
| `common.bad_request` | 400 | Generic bad request |
| `common.invalid_params` | 400 | Query/path params invalid |
| `common.validation_failed` | 400 | Request body validation failed |
| `common.required_field_missing` | 400 | DB not-null violation |
| `common.token_missing` | 401 | No bearer token in header |
| `common.token_invalid` | 401 | Token expired or invalid |
| `common.unauthorized` | 401 | Generic auth failure |
| `common.forbidden` | 403 | Role not allowed |
| `common.record_not_found` | 404 | Generic DB record not found |
| `common.route_not_found` | 404 | No matching route |
| `common.record_already_exists` | 409 | DB unique violation |
| `common.stale_version` | 409 | Optimistic lock conflict |
| `common.related_record_not_found` | 422 | DB foreign key violation |
| `common.rate_limited` | 429 | Too many requests |
| `common.internal_error` | 500 | Fallback for unknown errors |
