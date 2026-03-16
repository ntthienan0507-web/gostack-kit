# Design Patterns — When to use what

All patterns are already implemented in the template. Read before coding — don't reinvent solutions for problems that have already been solved.

---

## 1. Repository Pattern

**Problem:** Business logic is coupled with the database driver. Switching from SQLC to GORM requires modifying the service.

**Solution:** Interface in `repository.go`, implementation in `repository_gorm.go` / `repository_sqlc.go`.

```go
// Service only knows the interface — doesn't know whether GORM or SQLC is used
type Repository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
    Create(ctx context.Context, tx *gorm.DB, order *Order) (*Order, error)
}
```

**When to use:** All database access.
**When NOT to use:** In-memory data, config lookup.

---

## 2. Dependency Injection

**Problem:** Global variables, hidden dependencies, untestable code.

**Solution:** Constructor injection. All dependencies are passed through `New*()`.

```go
func NewService(repo Repository, db *gorm.DB, cache *cache.Client, workers *async.WorkerPool, logger *zap.Logger) *Service
```

**When to use:** All structs that have dependencies.
**When NOT to use:** Pure functions that don't need state.

**File:** `pkg/app/app.go` — wiring point, creates all dependencies and injects them.

---

## 3. Outbox Pattern

**Problem:** Dual-write — DB commit succeeds but Kafka publish fails (or vice versa). Data inconsistency.

**Solution:** Write events to the `outbox` table in the SAME transaction as business data. A relay process polls and publishes.

```go
database.WithTransaction(ctx, db, logger, func(tx *gorm.DB) error {
    tx.Create(&order)
    return broker.WriteOutbox(tx, TopicOrderCreated, order.ID.String(), event)
})
```

**When to use:** Whenever a business write needs to publish an event.
**When NOT to use:** Fire-and-forget notifications (use worker pool).

**Files:** `pkg/broker/outbox.go`, `pkg/broker/relay.go`

---

## 4. Dead Letter Queue (DLQ)

**Problem:** Kafka consumer handler fails → message is lost or retries infinitely.

**Solution:** Retry N times → send to `<topic>.dlq` with error context → ops investigate.

```go
consumer.Subscribe(topic, broker.WithDLQ(dlq, broker.DefaultDLQConfig(), handler))
```

**When to use:** All Kafka consumers.
**When NOT to use:** Never — always wrap with DLQ.

**Permanent error** (skip retry, send directly to DLQ):
```go
return broker.Permanent(fmt.Errorf("invalid payload: missing field"))
```

**File:** `pkg/broker/dlq.go`

---

## 5. Cache-Aside

**Problem:** Every read query hits the DB → slow, DB overload.

**Solution:** Check cache first → on miss, query DB → populate cache → return.

```
Read:  cache.Get → miss → DB query → cache.Set → return
Write: DB update → cache.Delete (invalidate)
```

**When to use:** Read-heavy endpoints, infrequently changing data (user profile, order detail).
**When NOT to use:** Constantly changing data (realtime dashboard), write-heavy.

**File:** `modules/order/service.go` (GetByID method)

---

## 6. Tag-based Cache Invalidation

**Problem:** Order response contains user info. User updates profile → all cached orders become stale. But we don't know which orders contain which user.

**Solution:** Tag each cache entry → invalidate by tag.

```go
// Cache order, tagged with the related user
cache.SetJSONWithTags(ctx, "order:123", data, 5*time.Minute, "user:456", "orders")

// User updates → delete all related caches
cache.InvalidateByTag(ctx, "user:456")
```

**When to use:** Cached data with relationships across entities.
**When NOT to use:** Cache key with 1-1 mapping (use Delete directly).

**File:** `pkg/cache/invalidation.go`

---

## 7. Optimistic Locking

**Problem:** 2 users edit the same record simultaneously, the latter overwrites the former without knowing.

**Solution:** `version` column. UPDATE only succeeds if the version matches.

```go
result := tx.Where("id = ? AND version = ?", id, version).
    Updates(map[string]any{"name": newName, "version": gorm.Expr("version + 1")})
if result.RowsAffected == 0 {
    return apperror.ErrStaleVersion  // 409 Conflict
}
```

**When to use:** Update user profile, order details, settings. Conflict is rare (<5%).
**When NOT to use:** New inserts. Frequent conflicts (use pessimistic lock).

---

## 8. Pessimistic Locking

**Problem:** Read → compute → write must be atomic. Example: deducting stock, 2 requests both read stock=1, both deduct → stock=-1.

**Solution:** `SELECT ... FOR UPDATE` locks the row within the transaction.

```go
database.WithTransaction(ctx, db, logger, func(tx *gorm.DB) error {
    tx.Clauses(database.ForUpdate()).Where("id = ?", id).First(&product)
    if product.Stock < qty {
        return ErrInsufficientStock
    }
    return tx.Model(&product).Update("stock", product.Stock - qty).Error
})
```

| Variant | Use case |
|---------|----------|
| `ForUpdate()` | Deduct balance, deduct stock |
| `ForUpdateNoWait()` | Payment — fail fast |
| `ForUpdateSkipLocked()` | Job queue — skip locked rows |
| `ForShare()` | Report — consistent read |

**When to use:** Read-modify-write critical paths, frequent conflicts, short transactions.
**When NOT to use:** Update descriptive fields (name, note). Long-running operations.

**File:** `pkg/database/lock.go`

---

## 9. Distributed Lock

**Problem:** 3 instances run a cron job → 3 emails sent. Only 1 should run.

**Solution:** Redis `SET NX` — only 1 caller wins. Lua script ensures only the owner releases.

```go
lock, err := distlock.Acquire(ctx, redisClient, "cron:monthly-report", distlock.Config{
    TTL: 30 * time.Second,
})
if errors.Is(err, distlock.ErrLockNotAcquired) {
    return  // another instance is running
}
defer lock.Release(ctx)
```

**When to use:** Cron job, webhook dedup, one-time migration.
**When NOT to use:** Single instance (use `sync.Mutex`). DB row lock (use `FOR UPDATE`).

**File:** `pkg/distlock/distlock.go`

---

## 10. Circuit Breaker

**Problem:** External service goes down → app keeps calling → thread pool exhausted → app also goes down.

**Solution:** After N failures → circuit "opens" → reject immediately without calling the service → periodically retry.

```
Closed (OK) → 5 failures → Open (reject all) → 30s → Half-Open (try 1 request)
                                                         ↓ success → Closed
                                                         ↓ fail → Open
```

```go
client := httpclient.New(httpclient.Config{
    BaseURL:        "https://payment.internal",
    CircuitBreaker: circuitbreaker.New(circuitbreaker.DefaultConfig("payment"), logger),
})
```

**When to use:** External HTTP calls (payment, email, 3rd party API).
**When NOT to use:** Database (has connection pool with auto retry). Internal calls within the same cluster.

**File:** `pkg/circuitbreaker/circuitbreaker.go`

---

## 11. Worker Pool (Fire-and-Forget)

**Problem:** Sending email in HTTP handler → slow response. Spawning uncontrolled goroutines → memory leak.

**Solution:** Fixed pool of workers + buffered queue. Submit task, don't wait for result.

```go
workers.Submit(func(ctx context.Context) error {
    return sendEmail(ctx, userID, email)
})
```

**When to use:** Notifications, audit log, webhook delivery, cache warming.
**When NOT to use:** Need result immediately (use `async.Parallel`). Need guaranteed delivery (use outbox + Kafka).

**IMPORTANT:** Do not capture `*gin.Context` in closures — copy values out first.

**File:** `pkg/async/worker.go`

---

## 12. Parallel Execution

**Problem:** Calling 3 services sequentially = 300ms. Calling in parallel = 100ms.

**Solution:**

```go
// Don't need results — just need all to finish
err := async.Parallel(ctx,
    func(ctx context.Context) error { return svcA.Sync(ctx) },
    func(ctx context.Context) error { return svcB.Sync(ctx) },
)

// Need results — collect, cancel on first error
results, err := async.ParallelCollect(ctx,
    func(ctx context.Context) (User, error) { return userSvc.Get(ctx, id) },
    func(ctx context.Context) (User, error) { return cacheSvc.Get(ctx, id) },
)
```

**When to use:** Aggregation endpoints, fan-out queries, independent service calls.
**When NOT to use:** Sequential dependencies (B needs A's result).

**File:** `pkg/async/parallel.go`

---

## 13. Token Source (Strategy Pattern)

**Problem:** HTTP client calling internal service needs auth. There are many ways to get a token: static key, OAuth2, forward user token.

**Solution:** `TokenSource` interface — client doesn't know where the token comes from.

```go
// Static API key
httpclient.Config{TokenSource: httpclient.NewStaticToken("sk_xxx")}

// OAuth2 client_credentials (auto refresh)
httpclient.Config{TokenSource: httpclient.NewClientCredentials(ccConfig)}

// Forward user's token from context
httpclient.Config{TokenSource: httpclient.NewTokenForwarder()}
```

**When to use:** External/internal HTTP calls that need authentication.

**File:** `pkg/httpclient/token.go`

---

## 14. Middleware Chain

**Problem:** Cross-cutting concerns (auth, logging, metrics) repeated in every handler.

**Solution:** Gin middleware stack — each middleware handles 1 concern.

```
Request → Recovery → RequestID → SecurityHeaders → Metrics → Tracing → CORS → RateLimit → Auth → Handler
```

| Middleware | File | Scope |
|-----------|------|-------|
| Recovery | `recovery.go` | Global — catch panics |
| RequestID | `requestid.go` | Global — generate/propagate trace ID |
| SecurityHeaders | `security.go` | Global — HSTS, CSP, X-Frame |
| Metrics | `metrics.go` | Global — Prometheus counters |
| Tracing | `tracing.go` | Global — OpenTelemetry spans |
| CORS | `cors.go` | Global — cross-origin |
| RateLimit | `ratelimit.go` | Global or per-route |
| Auth | `auth.go` | Per-group — validate Bearer token |
| RequireRole | `auth.go` | Per-route — RBAC check |
| Timeout | `timeout.go` | Per-route — deadline enforcement |
| MaxBodySize | `upload.go` | Per-route — upload size limit |
| AllowedFileTypes | `upload.go` | Per-route — MIME validation |

**When to create new middleware:** Concern repeats across 3+ routes.

### gRPC Interceptors (parallel to HTTP middleware)

gRPC uses interceptors instead of Gin middleware. Same concepts, different API:

| HTTP Middleware | gRPC Interceptor | File |
|----------------|-------------------|------|
| `middleware.Recovery` | `grpcserver.RecoveryInterceptor` | `pkg/grpcserver/interceptors.go` |
| `middleware.RequestLogger` | `grpcserver.LoggingInterceptor` | `pkg/grpcserver/interceptors.go` |
| `middleware.Auth` | `grpcserver.AuthInterceptor` | `pkg/grpcserver/interceptors.go` |
| `middleware.RequireRole` | `grpcserver.RequireRoleInterceptor` | `pkg/grpcserver/interceptors.go` |
| `response.HandleError` | `grpcserver.StatusFromError` | `pkg/grpcserver/errors.go` |

```go
// Claims extraction — same pattern, different context type
// HTTP:  claims, ok := middleware.GetClaims(ginCtx)
// gRPC:  claims, ok := grpcserver.ClaimsFromContext(ctx)
```

---

## 15. Feature Flags

**Problem:** Deploy new code but don't want to enable it for all users yet. Rollback = redeploy.

**Solution:** Config-driven flags. Toggle with env var, no redeploy needed.

```go
// Check in service
if featureManager.IsEnabled("beta_api") { ... }

// Guard route
router.GET("/beta/feature", featureManager.RequireFlag("beta_api"), handler)

// Maintenance mode
router.Use(featureManager.MaintenanceMiddleware())
```

**When to use:** New feature rollout, A/B testing, maintenance mode.
**When NOT to use:** Permanent logic branches (use code, not flags).

**File:** `pkg/feature/feature.go`

---

## 16. Audit Trail

**Problem:** Compliance requires knowing WHO did WHAT to WHICH resource WHEN.

**Solution:** Async audit log via worker pool — does not block HTTP response.

```go
auditLogger.LogFromGin(ctx, audit.ActionUpdate, "order", order.ID.String(), changes)
```

**When to use:** CUD operations on sensitive data (user, order, payment, permission changes).
**When NOT to use:** Read operations (unless compliance requires view logging).

**File:** `pkg/audit/audit.go`

---

## 17. Scheduled Jobs (Cron)

**Problem:** Recurring tasks (cleanup, reports) need to run periodically. Multiple instances → only 1 should run.

**Solution:** Cron scheduler + distributed lock.

```go
scheduler.Register(scheduler.Job{
    Name:     "cleanup-expired-tokens",
    Schedule: "0 2 * * *",  // daily 2am
    Fn:       cleanupExpiredTokens,
    Timeout:  5 * time.Minute,
})
```

**When to use:** Periodic cleanup, report generation, health checks.
**When NOT to use:** One-off tasks (use worker pool). Event-driven (use Kafka consumer).

**File:** `pkg/scheduler/scheduler.go`

---

## 18. Table Partitioning

**Problem:** Outbox table grows infinitely. DELETEing old rows = slow + vacuum + bloat.

**Solution:** Range partition by month. DROP partition = instant.

```
outbox (partitioned by created_at)
├── outbox_2026_01  ← DROP instant when all published
├── outbox_2026_02
├── outbox_2026_03  ← current (relay polls here)
└── outbox_2026_04  ← pre-created
```

**When to use:** High-write tables that need periodic cleanup (outbox, logs, events).
**When NOT to use:** Small tables. Tables that need frequent random access across all time ranges.

**File:** `db/migrations/00003_create_outbox.sql`

---

## 19. Dead Letter Table (DB-level)

**Problem:** Partition cleanup DROPs the table → failed outbox messages are lost.

**Solution:** Move failed rows → `outbox_failed` table before DROPping the partition.

```
Old outbox partition
  ├── published rows → DROP
  ├── failed rows → MOVE to outbox_failed → then DROP
  └── pending rows → SKIP (still processing)
```

**When to use:** Any partitioned table that has non-terminal state rows.

**File:** `db/migrations/00003_create_outbox.sql` (functions: `drop_old_outbox_partitions`)

---

## Quick Reference — "I need..."

| Need | Use pattern | Package |
|------|------------|---------|
| Call DB | Repository (#1) | `modules/*/repository.go` |
| Multiple tables in 1 atomic write | Transaction + Outbox (#3) | `pkg/database/transaction.go` |
| Publish event after write | Outbox (#3) | `pkg/broker/outbox.go` |
| Consume event safely | DLQ (#4) | `pkg/broker/dlq.go` |
| Cache read query | Cache-Aside (#5) | `pkg/cache/redis.go` |
| Invalidate related caches | Tag Invalidation (#6) | `pkg/cache/invalidation.go` |
| Prevent lost update | Optimistic Lock (#7) | `version` column |
| Deduct stock/balance atomically | Pessimistic Lock (#8) | `pkg/database/lock.go` |
| Cron runs on only 1 instance | Distributed Lock (#9) | `pkg/distlock/distlock.go` |
| External service resilience | Circuit Breaker (#10) | `pkg/circuitbreaker/` |
| Async non-blocking task | Worker Pool (#11) | `pkg/async/worker.go` |
| Call N services concurrently | Parallel (#12) | `pkg/async/parallel.go` |
| HTTP auth dynamic | Token Source (#13) | `pkg/httpclient/token.go` |
| Cross-cutting concern | Middleware (#14) | `pkg/middleware/` |
| Toggle feature at runtime | Feature Flags (#15) | `pkg/feature/feature.go` |
| Track who did what | Audit (#16) | `pkg/audit/audit.go` |
| Recurring job | Scheduler (#17) | `pkg/scheduler/scheduler.go` |
| Cleanup high-write table | Partitioning (#18) | `db/migrations/` |
| Expose gRPC API | gRPC server + interceptors | `pkg/grpcserver/` |
| gRPC error response | StatusFromError (#14) | `pkg/grpcserver/errors.go` |
