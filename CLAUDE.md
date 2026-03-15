# CLAUDE.md — AI Context for Go API Template

This file is read by AI coding assistants (Claude Code, Cursor, Copilot, etc.) before generating code.
Follow these rules strictly. Violating them creates tech debt that humans have to clean up.

## Project Overview

Production Go API template. Clean architecture, multi-DB support, Kafka events, full observability.

- **Language:** Go 1.25
- **Framework:** Gin
- **Database:** PostgreSQL (SQLC or GORM), MongoDB optional
- **Cache:** Redis
- **Events:** Kafka (Sarama) with outbox pattern
- **Auth:** JWT or Keycloak
- **Observability:** Prometheus + Jaeger (OpenTelemetry) + zap logging

## Architecture Rules — MUST follow

### Dependency direction (one-way only)

```
controller → service → repository → database
     ↓           ↓          ↓
  apperror    apperror    apperror
```

- `modules/` imports `pkg/` — NEVER the reverse
- Module A does NOT import Module B — communicate via Kafka events
- No global state — everything injected via constructors

### Module structure (copy this exactly for new modules)

```
modules/<name>/
├── models.go          — domain struct (NO gorm/json tags)
├── types.go           — DTOs: CreateRequest, UpdateRequest, Response + ToResponse()
├── errors.go          — var ErrXxx = apperror.New(code, "module.error_key", "detail")
├── events.go          — var TopicXxx = broker.MustRegisterTopic("module.entity.action", "EventType")
├── repository.go      — interface (write methods accept tx *gorm.DB)
├── repository_gorm.go — GORM implementation (gormXxx structs are unexported)
├── service.go         — business logic (NO gin.Context, NO HTTP concerns)
├── controller.go      — thin: parse request → call service → write response
├── routes.go          — route registration with auth middleware
└── *_test.go          — tests (mock at repository level)
```

## Code Generation Rules

### Errors — ALWAYS use apperror

```go
// ✅ CORRECT
return nil, apperror.New(http.StatusNotFound, "order.not_found", "Order does not exist")

// ✅ CORRECT — predefined error
var ErrOrderNotFound = apperror.New(http.StatusNotFound, "order.not_found", "Order not found")

// ❌ WRONG — raw errors lose HTTP code and i18n key
return nil, fmt.Errorf("order not found")
return nil, errors.New("not found")

// ❌ WRONG — direct gin JSON response bypasses error format
ctx.JSON(404, gin.H{"error": "not found"})
```

### Controller — thin, NO business logic

```go
// ✅ CORRECT
func (c *Controller) GetByID(ctx *gin.Context) {
    id, err := uuid.Parse(ctx.Param("id"))
    if err != nil {
        apperror.Respond(ctx, ErrInvalidID)
        return
    }
    result, err := c.service.GetByID(ctx.Request.Context(), id)
    if err != nil {
        apperror.HandleError(ctx, err)
        return
    }
    response.OK(ctx, result)
}

// ❌ WRONG — business logic in controller
func (c *Controller) GetByID(ctx *gin.Context) {
    // DO NOT query DB here
    // DO NOT check permissions here
    // DO NOT call cache here
}
```

### Service — NO HTTP, NO gin.Context

```go
// ✅ CORRECT — receives context.Context
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Response, error)

// ❌ WRONG — coupled to HTTP framework
func (s *Service) Create(ctx *gin.Context, req CreateRequest) (*Response, error)
```

### Repository — domain types only, NO ORM leaks

```go
// ✅ CORRECT — returns domain model
func (r *repo) GetByID(ctx context.Context, id uuid.UUID) (*Order, error)

// ❌ WRONG — leaks GORM model
func (r *repo) GetByID(ctx context.Context, id uuid.UUID) (*gormOrder, error)

// ❌ WRONG — leaks SQLC model
func (r *repo) GetByID(ctx context.Context, id uuid.UUID) (*db.User, error)
```

### Transactions — service layer owns, use outbox for events

```go
// ✅ CORRECT — atomic write + event
database.WithTransaction(ctx, s.db, s.logger, func(tx *gorm.DB) error {
    s.repo.Create(ctx, tx, order)
    return broker.WriteOutbox(tx, TopicOrderCreated, order.ID.String(), event)
})

// ❌ WRONG — dual write (DB commit + Kafka publish separately)
s.repo.Create(ctx, nil, order)
producer.Publish(ctx, topic, key, event)  // if this fails, order exists but event lost
```

### Database — required columns for every table

```sql
CREATE TABLE <name> (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    version     INT            NOT NULL DEFAULT 1,       -- optimistic locking
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ                              -- soft delete (NULL = active)
);
-- Partial index for soft delete
CREATE INDEX idx_<name>_xxx ON <name>(xxx) WHERE deleted_at IS NULL;
```

### Testing patterns

```go
// Mock at repository level using testify/mock
func newTestService() (*Service, *MockRepository) {
    repo := new(MockRepository)
    svc := NewService(repo, nil, nil, nil, zap.NewNop())
    return svc, repo
}

// HTTP tests use httptest + gin.TestMode
func init() { gin.SetMode(gin.TestMode) }

// Test naming: Test<Type>_<Method>_<Scenario>
func TestService_Create_Success(t *testing.T)
func TestController_GetByID_NotFound(t *testing.T)
```

### Logging — structured only

```go
// ✅ CORRECT
logger.Info("order created", zap.String("order_id", id), zap.Error(err))

// ❌ WRONG
logger.Info(fmt.Sprintf("order %s created", id))
log.Println("order created")
fmt.Println("debug:", err)
```

## What NOT to generate

- Do NOT use `log.Fatal` except in `main.go` startup
- Do NOT use `panic` for error handling
- Do NOT use `init()` except for broker topic registration
- Do NOT use `interface{}` — use `any` (Go 1.18+)
- Do NOT use GORM AutoMigrate — migrations are managed by goose
- Do NOT hardcode config values — use `pkg/config`
- Do NOT create files in `db/sqlc/` — they are auto-generated
- Do NOT add comments that restate the code — only comment WHY, not WHAT
- Do NOT add error handling for impossible cases in internal code
- Do NOT create abstractions for single-use code

## Key packages to use

| Need | Use | NOT |
|------|-----|-----|
| HTTP response | `response.OK()`, `response.Created()` | `ctx.JSON()` directly |
| Error response | `apperror.HandleError()`, `apperror.Respond()` | `ctx.JSON(500, ...)` |
| Transactions | `database.WithTransaction()` | `db.Begin()` manually |
| Background tasks | `workers.Submit(func(ctx) error { ... })` | `go func() { ... }()` |
| Parallel calls | `async.Parallel()`, `async.ParallelCollect()` | Manual WaitGroup |
| Cache | `cache.GetJSON()`, `cache.SetJSON()` | Direct Redis commands |
| Kafka publish | `broker.WriteOutbox()` inside TX | `producer.Publish()` directly |
| Kafka consume | `broker.WithDLQ(dlq, cfg, handler)` | Raw handler without retry |
| pgtype convert | `types.SetText()`, `types.CoerceText()` | Manual pgtype struct |
| Distributed lock | `distlock.Acquire()` | Manual Redis SET NX |
| Pessimistic lock | `database.ForUpdate()` | Raw `SELECT ... FOR UPDATE` string |
| Scheduled jobs | `scheduler.Register(job)` | Manual goroutine + time.Ticker |

## File locations

```
pkg/app/          — app bootstrap; services_*.go files are optional (deleted by cmd/init)
pkg/apperror/     — error types + common errors
pkg/response/     — HTTP response helpers
pkg/database/     — DB connections, transactions, locks
pkg/cache/        — Redis client + invalidation
pkg/broker/       — Kafka producer/consumer/outbox/DLQ
pkg/types/        — pgtype converters (Set*, Extract*, Coerce*)
pkg/auth/         — JWT/Keycloak providers
pkg/middleware/    — all HTTP middleware
pkg/async/        — worker pool, parallel execution
pkg/httpclient/   — external HTTP calls with retry/circuit breaker
pkg/distlock/     — distributed Redis lock
pkg/storage/      — S3 client + events
pkg/imaging/      — image processing (resize, WebP)
pkg/audit/        — audit logging
pkg/scheduler/    — cron job scheduler
pkg/feature/      — feature flags
pkg/i18n/         — error message translation
pkg/tracing/      — OpenTelemetry setup
pkg/metrics/      — Prometheus metrics
```
