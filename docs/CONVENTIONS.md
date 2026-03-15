# Code Conventions

Rules for the team. Read before writing code.

## Table of Contents

- [Project Structure](#project-structure)
- [Naming](#naming)
- [Error Handling](#error-handling)
- [Module Pattern](#module-pattern)
- [Repository Pattern](#repository-pattern)
- [Service Layer](#service-layer)
- [Controller Layer](#controller-layer)
- [Testing](#testing)
- [Database](#database)
- [Kafka Events](#kafka-events)
- [Configuration](#configuration)
- [Logging](#logging)
- [Git & PR](#git--pr)

---

## Project Structure

```
pkg/           ← Reusable infrastructure (shared across modules)
modules/       ← Business domains (user, order, ...)
db/migrations/ ← SQL migrations (goose)
db/queries/    ← SQLC query definitions
db/sqlc/       ← Generated code (DO NOT edit)
docs/          ← API docs, conventions
```

**Rules:**
- `pkg/` MUST NOT import `modules/` — dependency only goes one direction
- A module MUST NOT import another module — communicate via events or service interfaces
- No global state — everything is injected via constructor

---

## Naming

### Files

```
models.go              ← domain model (struct, no ORM tags)
types.go               ← DTOs: request, response, params
repository.go          ← interface
repository_gorm.go     ← GORM implementation
repository_sqlc.go     ← SQLC implementation
service.go             ← business logic
controller.go          ← HTTP handlers
routes.go              ← route registration
errors.go              ← module-specific errors
events.go              ← Kafka topic registration + event payloads
```

### Go

```go
// Package names: lowercase, single word
package user     // ✅
package userSvc  // ❌

// Interface names: action-oriented, no I prefix
type Repository interface {}  // ✅
type IRepository interface {} // ❌

// Error variables: Err prefix
var ErrNotFound = apperror.New(404, "user.not_found", "User not found")  // ✅
var NotFoundError = ...  // ❌

// Constructor: New + type name
func NewService(repo Repository, ...) *Service  // ✅
func CreateService(...) *Service                 // ❌

// Unexported: camelCase. Exported: PascalCase
func (s *Service) processOrder()  // ✅ internal
func (s *Service) ProcessOrder()  // ✅ exported
```

### Error Codes (apperror)

```
Format: <module>.<entity>.<action_or_state>

common.bad_request           ← shared errors
common.unauthorized
common.stale_version

user.not_found               ← module-specific
user.already_exists
order.not_cancellable
order.insufficient_stock
```

---

## Error Handling

### MUST use `apperror`

```go
// ✅ Typed error with HTTP code + i18n key
return nil, apperror.New(http.StatusNotFound, "order.not_found", "Order does not exist")

// ✅ Predefined module errors
var ErrOrderNotFound = apperror.New(http.StatusNotFound, "order.not_found", "Order not found")
return nil, ErrOrderNotFound

// ❌ Raw error — client does not know how to handle it
return nil, fmt.Errorf("order not found")

// ❌ Gin response directly — inconsistent format
ctx.JSON(404, gin.H{"error": "not found"})
```

### Controller error handling

```go
// ✅ Controller uses apperror.HandleError — automatically maps error → HTTP response
user, err := c.service.GetByID(ctx.Request.Context(), id)
if err != nil {
    apperror.HandleError(ctx, err)
    return
}

// ❌ Controller manually switches on error type
if errors.Is(err, pgx.ErrNoRows) {  // ← leaking DB concern into controller
    ctx.JSON(404, ...)
}
```

---

## Module Pattern

Each module is a single business domain. Follow this structure:

```
modules/order/
├── models.go          ← Domain model (no ORM tags, no JSON tags)
├── types.go           ← DTOs + ToResponse mapper
├── errors.go          ← var ErrXxx = apperror.New(...)
├── events.go          ← var TopicXxx = broker.MustRegisterTopic(...)
├── repository.go      ← interface
├── repository_gorm.go ← implementation
├── service.go         ← business logic
├── controller.go      ← HTTP handlers
├── routes.go          ← route registration
└── *_test.go          ← tests
```

### Domain Model vs GORM Model vs DTO

```go
// models.go — DOMAIN model. No tags. Used in service layer.
type Order struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    Status     OrderStatus
    TotalPrice float64
}

// repository_gorm.go — GORM model. Internal to repository. Never exported.
type gormOrder struct {
    ID     uuid.UUID      `gorm:"type:uuid;primaryKey"`
    Status string         `gorm:"type:varchar(50)"`
    // ...
}

// types.go — DTO. JSON tags. Sent to/from client.
type OrderResponse struct {
    ID     uuid.UUID `json:"id"`
    Status string    `json:"status"`
}
```

**Rule:** ORM types MUST NOT leak outside the repository. Service/Controller only sees domain models and DTOs.

---

## Repository Pattern

### Interface in `repository.go`

```go
type Repository interface {
    List(ctx context.Context, params ListParams, limit, offset int32) ([]*Order, error)
    Count(ctx context.Context, params ListParams) (int64, error)
    GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
    Create(ctx context.Context, tx *gorm.DB, order *Order) (*Order, error)  // tx for transaction
    UpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, status OrderStatus, version int) error
    SoftDelete(ctx context.Context, id uuid.UUID) error
}
```

**Rules:**
- Write methods accept a `tx *gorm.DB` parameter — allows participating in an external transaction
- Read methods DO NOT accept tx — use the default connection
- Return domain models, DO NOT return GORM models
- `context.Context` is always the first parameter

### Transaction-aware pattern

```go
// Repository method accepts tx or uses default db
func (r *gormRepository) conn(tx *gorm.DB) *gorm.DB {
    if tx != nil {
        return tx
    }
    return r.db
}

func (r *gormRepository) Create(ctx context.Context, tx *gorm.DB, order *Order) (*Order, error) {
    if err := r.conn(tx).WithContext(ctx).Create(&row).Error; err != nil {
        return nil, err
    }
    // ...
}
```

---

## Service Layer

### Dependencies are injected via constructor

```go
type Service struct {
    repo    Repository
    db      *gorm.DB          // for transactions
    cache   *cache.Client     // optional (nil = no cache)
    workers *async.WorkerPool // for fire-and-forget tasks
    logger  *zap.Logger
}

func NewService(repo Repository, db *gorm.DB, cache *cache.Client, workers *async.WorkerPool, logger *zap.Logger) *Service
```

### Rules

```go
// ✅ Service MUST NOT know about HTTP (no gin.Context, no request/response types)
func (s *Service) Create(ctx context.Context, req CreateOrderRequest) (*OrderResponse, error)

// ❌ Service accepts gin.Context — coupling with HTTP framework
func (s *Service) Create(ctx *gin.Context, req CreateOrderRequest)

// ✅ Transaction at service layer
database.WithTransaction(ctx, s.db, s.logger, func(tx *gorm.DB) error {
    s.repo.Create(ctx, tx, order)
    broker.WriteOutbox(tx, topic, key, event)
    return nil
})

// ❌ Transaction at controller or repository
```

---

## Controller Layer

### Thin — only parse request, call service, write response

```go
func (c *Controller) Create(ctx *gin.Context) {
    // 1. Parse + validate request
    var req CreateRequest
    if err := ctx.ShouldBindJSON(&req); err != nil {
        apperror.Respond(ctx, apperror.New(400, "common.validation_failed", err.Error()))
        return
    }

    // 2. Call service
    result, err := c.service.Create(ctx.Request.Context(), req)
    if err != nil {
        apperror.HandleError(ctx, err)
        return
    }

    // 3. Write response
    response.Created(ctx, result)
}
```

**DO NOT do in controller:**
- Business logic
- Database queries
- Cache operations
- Kafka publishing

---

## Testing

### File naming

```
service_test.go          ← unit tests for service
controller_test.go       ← HTTP handler tests
mock_repository_test.go  ← testify mock for repository interface
```

### Patterns

```go
// init() — set gin test mode
func init() { gin.SetMode(gin.TestMode) }

// Helper — create service + mock
func newTestService() (*Service, *MockRepository) {
    repo := new(MockRepository)
    svc := NewService(repo, nil, nil, nil, zap.NewNop())
    return svc, repo
}

// Helper — create test router
func setupRouter(ctrl *Controller) *gin.Engine {
    r := gin.New()
    g := r.Group("/api/v1")
    // register routes...
    return r
}

// Test naming: Test<Method>_<Scenario>
func TestService_Create_Success(t *testing.T)
func TestService_Create_ValidationError(t *testing.T)
func TestController_GetByID_NotFound(t *testing.T)
```

### Rules
- Mock at repository level, DO NOT mock service
- Use `testify/mock` for mocks, `testify/assert` for assertions
- Use `httptest.NewRecorder` for controller tests
- Use `zap.NewNop()` for logger
- Test coverage > 70% for service layer

---

## Database

### Migrations

```sql
-- File: db/migrations/00005_create_invoices.sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS invoices (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    version     INT            NOT NULL DEFAULT 1,       -- optimistic locking
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ                              -- soft delete
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS invoices;
-- +goose StatementEnd
```

**Rules:**
- Every table MUST have: `id`, `created_at`, `updated_at`, `deleted_at`, `version`
- Use `UUID` for primary key, `gen_random_uuid()` for default
- Use `TIMESTAMPTZ` (not TIMESTAMP) — timezone-aware
- Partial index for soft delete: `WHERE deleted_at IS NULL`
- Migration MUST have both Up and Down
- DO NOT use GORM AutoMigrate

### Soft Delete

All business data tables MUST use soft delete. Hard delete only for:
- Processed events (cleanup after 7 days)
- Cache entries
- Temporary data

---

## Kafka Events

### Topic registration

```go
// modules/order/events.go
var (
    TopicOrderCreated   = broker.MustRegisterTopic("order.order.created", "OrderCreatedEvent")
    TopicOrderCancelled = broker.MustRegisterTopic("order.order.cancelled", "OrderCancelledEvent")
)
```

**Naming:** `<module>.<entity>.<past_tense_action>`

### Event struct

```go
type OrderCreatedEvent struct {
    OrderID    uuid.UUID `json:"order_id"`
    UserID     uuid.UUID `json:"user_id"`
    TotalPrice float64   `json:"total_price"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### Publishing — ALWAYS via outbox

```go
// ✅ Outbox pattern — event in the same transaction as business data
database.WithTransaction(ctx, db, logger, func(tx *gorm.DB) error {
    tx.Create(&order)
    return broker.WriteOutbox(tx, TopicOrderCreated, order.ID.String(), event)
})

// ❌ Direct publish — dual-write problem
tx.Create(&order)
producer.Publish(ctx, topic, key, event)  // if publish fails → order is created but event is lost
```

### Consuming — ALWAYS use DLQ wrapper

```go
// ✅ Retry + DLQ
consumer.Subscribe(topic, broker.WithDLQ(dlq, broker.DefaultDLQConfig(), handler))

// ❌ Raw handler — error = message lost
consumer.Subscribe(topic, handler)
```

---

## Configuration

- All config via environment variables (12-factor)
- Default values in `config.Load()`
- Sensitive values (password, secret, key) MUST NOT be committed to git
- Use `.env` for local dev, K8s secrets for production

---

## Logging

```go
// ✅ Structured logging — searchable fields
logger.Info("order created",
    zap.String("order_id", order.ID.String()),
    zap.String("user_id", order.UserID.String()),
    zap.Float64("total", order.TotalPrice),
)

// ❌ String interpolation — not searchable
logger.Info(fmt.Sprintf("order %s created by %s", order.ID, order.UserID))

// ✅ Error logging — include error + context
logger.Error("create order failed",
    zap.String("user_id", userID),
    zap.Error(err),
)

// ❌ Bare error
logger.Error(err.Error())
```

**Log levels:**
- `Debug` — development only (query details, cache hits)
- `Info` — business events (order created, user logged in)
- `Warn` — recoverable issues (cache miss, retry)
- `Error` — failures (DB error, external service down)
- `Fatal` — startup failures only (can't connect to DB)

---

## Git & PR

### Branch naming

```
feature/add-invoice-module
fix/order-cancel-race-condition
chore/upgrade-go-1.25
```

### Commit message

```
feat: add invoice module with CRUD + Kafka events
fix: race condition in order cancellation (optimistic lock)
chore: upgrade Go 1.24 → 1.25
docs: add database patterns guide
test: add order service unit tests
```

### PR checklist

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] `golangci-lint run ./...` clean
- [ ] Migration has both Up and Down
- [ ] New module follows module pattern
- [ ] Error codes registered in `errors.go`
- [ ] Kafka topics registered in `events.go`
- [ ] Tests cover happy path + error cases
