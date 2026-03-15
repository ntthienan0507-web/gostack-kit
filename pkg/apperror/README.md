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
| `error_detail` | `string` | Human-readable description for debugging |

## Architecture

```
pkg/apperror/
  error.go       # AppError type, Abort(), Respond(), HandleError(), FromError()
  common.go      # Shared errors: common.* namespace (auth, validation, 500)

modules/<module>/
  errors.go      # Module-specific errors: <module>.* namespace
```

**Rule:** `common.go` is locked — only infrastructure errors live here (auth, validation, DB).
Each module owns its own `errors.go`. No cross-module error imports.

## How to Add Errors for a New Module

### 1. Create `modules/<module>/errors.go`

```go
package order

import (
    "net/http"

    "github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// Order module error codes.
// Namespace: "order.*"
var (
    ErrOrderNotFound     = apperror.New(http.StatusNotFound, "order.not_found", "Order does not exist")
    ErrOrderAlreadyPaid  = apperror.New(http.StatusConflict, "order.already_paid", "Order has already been paid")
    ErrInvalidOrderID    = apperror.New(http.StatusBadRequest, "order.invalid_id", "Invalid order ID format")
    ErrInsufficientStock = apperror.New(http.StatusUnprocessableEntity, "order.insufficient_stock", "Not enough stock to fulfill order")
)
```

### 2. Use in Controller

```go
func (c *Controller) GetByID(ctx *gin.Context) {
    id, err := uuid.Parse(ctx.Param("id"))
    if err != nil {
        // Module-specific error
        apperror.Respond(ctx, ErrInvalidOrderID)
        return
    }

    order, err := c.service.GetByID(ctx.Request.Context(), id)
    if err != nil {
        // Auto-maps: *AppError pass-through, pgx/mongo → common errors, unknown → 500
        apperror.HandleError(ctx, err)
        return
    }

    response.OK(ctx, order)
}
```

### 3. Use in Service (return AppError directly)

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

### 4. Override Detail with Context

```go
// Reuse the error key but add specific detail
return ErrInsufficientStock.WithDetail(
    fmt.Sprintf("Requested %d but only %d available", requested, available),
)
// → { "error_code": 422, "error_message": "order.insufficient_stock", "error_detail": "Requested 5 but only 2 available" }
```

## API Reference

| Function | Usage |
|----------|-------|
| `apperror.New(code, message, detail)` | Create a new AppError |
| `apperror.Respond(ctx, err)` | Write AppError as JSON response |
| `apperror.Abort(ctx, err)` | Write AppError + abort middleware chain (use in middleware) |
| `apperror.HandleError(ctx, err)` | Auto-convert any error → AppError → JSON response |
| `apperror.FromError(err)` | Convert any error → AppError (without writing response) |
| `err.WithDetail(detail)` | Copy error with custom detail message |

## Error Flow

```
Controller                          Service / Repository
    │                                       │
    │  ← return ErrOrderNotFound ───────────┘  (module-specific AppError)
    │  ← return pgx.ErrNoRows ─────────────┘  (raw DB error)
    │  ← return fmt.Errorf("wrap: %w", err) ┘  (wrapped error)
    │
    ├─ apperror.HandleError(ctx, err)
    │       │
    │       └─ FromError(err)
    │              ├─ *AppError?     → return as-is
    │              ├─ pgx.ErrNoRows? → common.record_not_found
    │              ├─ pgconn 23505?  → common.record_already_exists
    │              ├─ mongo dup key? → common.record_already_exists
    │              └─ unknown?       → common.internal_error
    │
    └─ JSON response written
```

## Naming Convention

```
<module>.<action_or_entity>_<reason>
```

Examples:
- `user.not_found`
- `user.already_exists`
- `user.invalid_id`
- `order.insufficient_stock`
- `payment.card_declined`
- `common.token_invalid`
- `common.forbidden`

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
| `common.related_record_not_found` | 422 | DB foreign key violation |
| `common.internal_error` | 500 | Fallback for unknown errors |
