# Architecture

## Project Structure

```
go-api-template/
├── main.go                          # CLI entry point: serve, migrate, db
├── db/
│   ├── migrations/                  # Goose SQL migrations
│   ├── queries/                     # SQLC query definitions
│   └── sqlc/                        # Generated SQLC code
│
├── modules/                         # Business modules (CRUD, routes, business logic)
│   ├── user/
│   │   ├── models.go                # Domain entity (User struct)
│   │   ├── types.go                 # Request/Response DTOs + transformers
│   │   ├── repository.go            # Repository interface
│   │   ├── repository_sqlc.go       # SQLC implementation
│   │   ├── repository_gorm.go       # GORM implementation
│   │   ├── repository_mongo.go      # MongoDB implementation
│   │   ├── service.go               # Business logic
│   │   ├── controller.go            # HTTP handlers (parse request → call service → respond)
│   │   ├── routes.go                # Route registration
│   │   ├── errors.go                # Module-specific AppError codes (user.*)
│   │   └── events.go                # Kafka topic registration + event structs
│   └── order/                       # Same structure as user
│
├── pkg/                             # Shared infrastructure — used by all modules
│   ├── app/                         # DI container + service registry + server lifecycle
│   ├── apperror/                    # Structured error handling (i18n-ready keys)
│   ├── async/                       # Worker pool + parallel execution + context safety
│   ├── auth/                        # Authentication providers (JWT, Keycloak, OAuth2, SAML)
│   ├── broker/                      # Kafka producer/consumer + outbox + dispatcher + batcher
│   ├── cache/                       # Redis cache abstraction
│   ├── circuitbreaker/              # Circuit breaker (optional, disabled by default)
│   ├── config/                      # Configuration (Viper, .env + env vars)
│   ├── cron/                        # Lightweight cron scheduler
│   ├── crypto/                      # AES-256 encryption + bcrypt + random tokens
│   ├── database/                    # Postgres + GORM + Mongo + Redis + transactions
│   ├── external/                    # 3rd party service clients
│   │   ├── sendgrid/                #   email (JSON)
│   │   ├── stripe/                  #   payments (JSON)
│   │   ├── icewarp/                 #   mail server (XML)
│   │   ├── firebase/                #   push notifications (gRPC/SDK)
│   │   └── elasticsearch/           #   search & analytics
│   ├── httpclient/                  # Base HTTP client (TLS, retry, codec, token mgmt)
│   ├── logger/                      # Structured logging (Zap)
│   ├── metrics/                     # Prometheus metrics
│   ├── grpcserver/                   # gRPC server, interceptors, error mapping
│   ├── middleware/                   # HTTP middleware (auth, validation, CORS, audit)
│   ├── response/                    # HTTP response helpers (success + error)
│   ├── retry/                       # Exponential backoff + jitter
│   ├── tracing/                     # OpenTelemetry distributed tracing
│   └── ws/                          # WebSocket (hub, rooms, message routing)
│
└── docs/                            # Generated Swagger docs
```

## Package Reference

### `pkg/app` — DI Container + Server Lifecycle

Wires all dependencies. No globals — everything injected via constructors.

```
Config → Database → Redis → Auth → Tracing → WorkerPool → Services → Router → Modules
```

**`app.go`** — DI container, server lifecycle, readiness probe
**`services.go`** — external service registry (auto-init from env, nil if unconfigured)

```go
// Services are nil-safe — unconfigured = nil = service not available
type Services struct {
    Encryptor     *crypto.Encryptor
    SendGrid      *sendgrid.Client
    Stripe        *stripe.Client
    IceWarp       *icewarp.Client
    Firebase      *firebase.Client
    Elasticsearch *elasticsearch.Client
    KafkaProducer broker.Producer
    KafkaConsumer broker.Consumer
}
```

**Key features:**
- Graceful shutdown: drain HTTP → flush batches → stop workers → close services → close DB/Redis
- Readiness probe: `/readyz` checks DB + Redis health
- Response audit middleware (debug mode only)
- Services auto-init: set env var = service enabled, empty = skipped

---

### `pkg/config` — Configuration

Viper-based. Reads `.env` file + environment variables (env overrides file).

```bash
./go-api-template serve    # reads .env + env vars
DB_DRIVER=gorm ./go-api-template serve  # env var overrides .env
```

All config is loaded once at startup, passed via constructors. No global vars.

---

### `pkg/database` — Database Connections

| File | What |
|------|------|
| `postgres.go` | pgxpool connection with health checks |
| `gorm.go` | GORM connection (PostgreSQL) |
| `mongo.go` | MongoDB connection with ping |
| `redis.go` | Redis connection with pool config |
| `store.go` | pgx transaction wrapper (`ExecTx`) |
| `transaction.go` | GORM transaction wrapper (`WithTransaction`, `WithTransactionResult[T]`) |
| `migration.go` | Goose migration runner |

**Switch database driver via env:**
```bash
DB_DRIVER=sqlc   # default — pgx + SQLC generated code
DB_DRIVER=gorm   # GORM ORM
DB_DRIVER=mongo  # MongoDB
```

---

### `pkg/auth` — Authentication Providers

Pluggable auth via `AUTH_PROVIDER` env var.

| Provider | When |
|----------|------|
| `jwt` | Self-hosted JWT signing/validation (default) |
| `keycloak` | Keycloak/OIDC token validation |
| `oauth2` | Generic OAuth2/OIDC (Google, Azure AD, Okta, Auth0) |
| `saml` | SAML 2.0 SP — enterprise SSO (ADFS, Okta, OneLogin) |

---

### `pkg/apperror` — Structured Error Handling (pure, no gin)

All errors follow:
```json
{"error_code": 404, "error_message": "user.not_found", "error_detail": "User with the given ID does not exist"}
```

In release mode, `error_detail` is stripped to prevent leaking internal info (SQL errors, file paths).

- `error_message`: snake_case key, namespaced by module — clients use for i18n
- `common.go`: shared errors (auth, validation, 500) — **locked, don't add module errors here**
- Each module: `errors.go` with `<module>.*` namespace

**Key functions:**
| Function | Use |
|----------|-----|
| `apperror.New(code, message, detail)` | Create an AppError |
| `apperror.FromError(err)` | Convert any error → AppError (pgx, mongo, etc.) |
| `err.WithDetail(detail)` | Copy error with custom detail |
| `err.Sanitize()` | Copy error with detail stripped (used in production) |

> `apperror` is transport-agnostic — no gin dependency. HTTP and gRPC layers handle response writing.

---

### `pkg/response` — HTTP Response Helpers

**Success responses:**
```json
{"status": "success", "data": {...}}
{"status": "success", "data": {"items": [...], "total": 10}}
```

**Error responses:** delegates to `apperror.AppError` struct, auto-sanitizes in release mode.

```go
// Success
response.OK(ctx, user)              // 200 + typed
response.OKList(ctx, users, total)  // 200 + list
response.Created(ctx, user)         // 201 + typed
response.NoContent(ctx)             // 204

// Error (gin-specific, in response/error.go)
response.Error(ctx, apperror.ErrNotFound)    // write AppError as JSON
response.Abort(ctx, apperror.ErrTokenMissing) // write + abort chain
response.HandleError(ctx, err)                // auto-convert → AppError → JSON
```

---

### `pkg/middleware` — HTTP Middleware

| Middleware | What |
|-----------|------|
| `Recovery` | Catch panics → 500 AppError |
| `RequestLogger` | Log method, path, status, latency (Zap) |
| `CORS` | CORS headers |
| `Auth` | Validate bearer token → set claims in context |
| `RequireRole` | Check user role (admin, user, etc.) |
| `ResponseAudit` | **Debug only** — warn when response bypasses standard format |
| `Metrics` | Prometheus request metrics |
| `RateLimit` | Token bucket per-IP (in-memory or Redis) |
| `Timeout` | Per-route request deadline |
| `ValidateJSON` | Validate request body + store in context |
| `MaxBodySize` | Upload size limit |
| `AllowedFileTypes` | MIME type validation |

> Middleware uses `abortWithAppError()` internally — does NOT depend on `pkg/response`.

---

### `pkg/grpcserver` — gRPC Server Infrastructure

Runs alongside HTTP on a separate port (`GRPC_PORT`, default 9090).

| File | What |
|------|------|
| `server.go` | Server factory with health check + reflection (debug mode) |
| `interceptors.go` | Unary + Stream: Recovery, Logging, Auth, RequireRole |
| `errors.go` | `StatusFromError()` — maps AppError → gRPC status codes |

```go
// gRPC handler extracts claims the same way
claims, ok := grpcserver.ClaimsFromContext(ctx)
```

**HTTP ↔ gRPC share the same service layer:**
```
HTTP :8080                          gRPC :9090
  │                                    │
  ├─ Gin middleware                     ├─ Unary/Stream interceptors
  ├─ controller.go ─┐                  ├─ grpc_handler.go ─┐
  │                  ├──→ service.go   │                    ├──→ service.go
  │                  │   (shared!)     │                    │   (shared!)
```

---

### `pkg/httpclient` — HTTP Client for Service-to-Service Calls

Base client with security defaults. All 3rd party services embed this.

| Feature | How |
|---------|-----|
| TLS 1.2+ | `MinVersion: tls.VersionTLS12` |
| Timeout | Default 10s, configurable |
| Body limit | 10MB max read |
| Auth leak on redirect | Strip `Authorization` on cross-origin redirect |
| Retry | Via `pkg/retry` — exponential backoff + jitter |
| Circuit breaker | Optional — via `pkg/circuitbreaker` |
| Source IP binding | For IP-whitelisted 3rd parties |
| Ping | Health check on startup |

**Codec support:**
| Codec | Content-Type | Used by |
|-------|-------------|---------|
| `JSONCodec` | `application/json` | SendGrid, Stripe (default) |
| `XMLCodec` | `application/xml` | IceWarp |

```go
// JSON service (default)
httpclient.NewServiceClient(c, httpclient.ServiceConfig{ErrorDecoder: &errorDecoder{}})

// XML service
httpclient.NewServiceClient(c, httpclient.ServiceConfig{
    ContentType:  httpclient.ContentXML,
    ErrorDecoder: &errorDecoder{},
})
```

**Token management:**
| TokenSource | Use case |
|-------------|----------|
| `StaticToken` | Fixed API key (SendGrid, Stripe) |
| `TokenForwarder` | Forward user's token to downstream |
| `ClientCredentials` | OAuth2 service-to-service (auto-refresh) |

---

### `pkg/external` — 3rd Party Service Clients

Every 3rd party follows this layout:
```
pkg/external/<service>/
  client.go    # Client struct + public methods
  types.go     # Request/Response DTOs matching 3rd party API
  errors.go    # ErrorDecoder + <service>.* AppError codes
```

| Service | API Format | Purpose |
|---------|-----------|---------|
| `sendgrid` | JSON | Email delivery |
| `stripe` | JSON | Payment processing |
| `icewarp` | XML | Email + mailbox management |
| `elasticsearch` | JSON | Search + analytics |
| `orderservice` | JSON | Internal service example |

---

### `pkg/retry` — Retry with Exponential Backoff

```go
retry.Do(ctx, retry.DefaultConfig, func() error { ... })
retry.DoWithResult[T](ctx, cfg, func() (T, error) { ... })
```

| Config | Default | Aggressive |
|--------|---------|-----------|
| MaxRetries | 3 | 5 |
| BaseDelay | 100ms | 50ms |
| MaxDelay | 10s | 30s |
| Multiplier | 2.0 | 2.0 |

**Retryable:** network errors, 5xx, 429
**Not retryable:** 4xx (except 429), context cancelled

---

### `pkg/circuitbreaker` — Circuit Breaker (Optional)

Wraps calls to external services. Opens after 5 consecutive failures → rejects immediately for 30s → half-open probe → close.

**Default disabled.** Enable per-service when needed:
```go
httpclient.Config{CircuitBreaker: circuitbreaker.New(...)}
broker.Config{EnableCircuitBreaker: true}
```

---

### `pkg/async` — Goroutine Management

**WorkerPool** — background task runner (emails, webhooks, audit logs):
```go
app.Workers.Submit(func(ctx context.Context) error {
    return sendEmail(ctx, userID, email)
})
```
- Panic-safe, drain on shutdown
- Backpressure via queue size
- Task receives background context (NOT gin.Context)

**Parallel** — fan-out helper:
```go
err := async.Parallel(ctx, taskA, taskB, taskC)
results, err := async.ParallelCollect[T](ctx, fnA, fnB, fnC)
```

**Context safety:**
```go
// ✅ Copy values from gin.Context BEFORE submitting
userID := ginCtx.GetString("user_id")
app.Workers.Submit(func(ctx context.Context) error {
    return doWork(ctx, userID)  // safe — userID is a copy
})

// ❌ NEVER capture gin.Context in goroutine closure
app.Workers.Submit(func(ctx context.Context) error {
    return doWork(ctx, ginCtx.GetString("user_id"))  // BUG — ginCtx recycled
})
```

---

### `pkg/broker` — Kafka Message Broker

**Producer:**
```go
producer.Publish(ctx, user.TopicUserCreated, userID, event)
```

**Consumer** (parallel, key-ordered):
```go
consumer.Subscribe(topic, handler)                           // per-message
consumer.SubscribeBatch(topic, batchHandler, BatchOpts{...}) // bulk processing
```

**Transactional Outbox** — guaranteed event delivery:
```go
database.WithTransaction(ctx, db, logger, func(tx *gorm.DB) error {
    tx.Create(&order)
    return broker.WriteOutbox(tx, order.TopicOrderCreated, order.ID.String(), event)
})
// Relay goroutine: poll outbox → publish to Kafka → mark done
```

**Topic registry** — typed, validated, no raw strings:
```go
var TopicOrderCreated = broker.MustRegisterTopic("order.order.created", "OrderCreatedEvent", broker.KeyByID)
```

**Idempotency** — prevent duplicate processing:
```go
consumer.Subscribe(topic, broker.IdempotentHandler(db, logger, myHandler))
```

**Key features:**
| Feature | Implementation |
|---------|---------------|
| Manual offset commit | Auto-commit disabled, commit after handler success |
| Key-sharded workers | Same key → same worker → ordering preserved |
| Batch processing | Accumulate → flush by size or timeout → bulk DB insert |
| Fetch tuning | `FetchMinBytes`, `FetchMaxWait` for throughput |
| Error classification | Retryable (timeout) vs permanent (too large) |
| Circuit breaker | Optional, stops hammering dead broker |
| SASL auth | PLAIN, SCRAM-SHA-256, SCRAM-SHA-512 |

---

### `pkg/cache` — Redis Cache Abstraction

Cache layer over Redis with typed get/set.

---

### `pkg/metrics` — Prometheus Metrics

Exposes `/metrics` endpoint for Prometheus scraping.

| Metric | Type |
|--------|------|
| `db_query_duration_seconds` | Histogram |
| `cache_misses_total` | Counter |
| `external_request_duration_seconds` | Histogram |

---

### `pkg/cron` — Job Scheduler

Lightweight cron scheduler — no external dependency.

```go
scheduler := cron.New(logger)
scheduler.Register("cleanup", "0 3 * * *", cleanupJob)    // daily 3AM
scheduler.Register("report",  "@every 1h",  reportJob)     // every hour
scheduler.Start(ctx)
```

Run as separate process: `./go-api-template cron`

---

### `pkg/crypto` — Encryption & Hashing

| Function | Use |
|----------|-----|
| `Encrypt/Decrypt` | AES-256-GCM for PII (email, phone) |
| `HashPassword/CheckPassword` | bcrypt for passwords |
| `SHA256Hex` | Deterministic hash |
| `RandomToken/RandomHex` | Secure random generation |

---

### `pkg/ws` — WebSocket

Hub-based WebSocket with rooms, message routing, and auth.

```go
hub := ws.NewHub(logger)
hub.Handle("chat.send", handleChat)
r.GET("/ws", ws.UpgradeHandler(hub, logger, authFunc))
```

Features: rooms, broadcast, direct message, ping/pong, backpressure, panic-safe handlers.

---

### `pkg/tracing` — OpenTelemetry Tracing

Distributed tracing via OpenTelemetry. Enable via env:
```bash
OTEL_ENABLED=true
OTEL_SERVICE_NAME=myapp
OTEL_ENDPOINT=localhost:4318
```

---

### `pkg/logger` — Structured Logging

Zap logger factory. No globals.

```bash
LOG_FORMAT=console  # human-readable (dev)
LOG_FORMAT=json     # structured (production)
LOG_LEVEL=debug     # debug | info | warn | error
```

---

## CLI Commands

```bash
# Project setup (interactive — pick stacks you need)
./scripts/init.sh
./scripts/init.sh --minimal    # DB + Redis only
./scripts/init.sh --all        # everything

# Start API server (auto-migrates, graceful shutdown)
./go-api-template serve

# Start cron scheduler (separate process)
./go-api-template cron

# Database migrations
./go-api-template migrate up              # apply pending
./go-api-template migrate down            # rollback one
./go-api-template migrate status          # show status
./go-api-template migrate create <name>   # create new migration
./go-api-template migrate version         # current version

# Database utilities
./go-api-template db test                 # test connection
./go-api-template db tables               # list tables
./go-api-template db columns <table>      # show columns
./go-api-template db query "SELECT ..."   # run SQL
./go-api-template db shell                # open psql

# Query performance
./go-api-template db explain "SELECT ..."  # EXPLAIN ANALYZE with buffers
./go-api-template db slow-queries          # top 20 slowest (pg_stat_statements)
./go-api-template db index-usage           # unused indexes eating disk
./go-api-template db table-stats           # sizes, dead tuples, vacuum status
./go-api-template db locks                 # who's blocking who
./go-api-template db connections           # connection pool health

# Outbox management
./go-api-template db outbox-maintain      # partition maintenance
./go-api-template db outbox-partitions    # list partitions
./go-api-template db outbox-failed        # show failed messages
./go-api-template db outbox-retry <id>    # retry one failed
./go-api-template db outbox-retry-all     # retry all failed
```

## Module Layout

Every business module follows this structure:

```
modules/<module>/
  models.go          # Domain entity
  types.go           # Request/Response DTOs + transformer
  repository.go      # Repository interface
  repository_*.go    # Implementations (sqlc, gorm, mongo)
  service.go         # Business logic
  controller.go      # HTTP handlers
  grpc_handler.go    # gRPC service implementation (optional)
  routes.go          # Route registration
  errors.go          # <module>.* AppError codes
  events.go          # Kafka topic registration + event structs
```

## 3rd Party Service Layout

Every external service follows this structure:

```
pkg/external/<service>/
  client.go          # Client struct + public methods
  types.go           # Request/Response DTOs (match 3rd party API format)
  errors.go          # ErrorDecoder + <service>.* AppError codes
```

## Request Flow

```
HTTP Request                              gRPC Request
     │                                         │
  Middleware (recovery → logger → CORS          Interceptors (recovery → logger
     → auth → audit)                              → auth)
     │                                         │
  Controller (parse → call service)       gRPC Handler (parse → call service)
     │                                         │
     └───────────────┬─────────────────────────┘
                     │
              Service (business logic, transactions, outbox events)
                     │
              Repository (data access — SQLC/GORM/Mongo)
                     │
                  Database
```

## Event Flow (Kafka)

```
Service.Create()
  │
  ├─ DB Transaction: INSERT data + INSERT outbox (atomic)
  │
  └─ WorkerPool: send notification (best-effort)

Outbox Relay (background goroutine)
  │
  └─ Poll outbox → Publish to Kafka → Mark published

Kafka Consumer (background goroutine)
  │
  ├─ Subscribe (per-message): real-time processing
  └─ SubscribeBatch (bulk): batch insert to ELK/reporting DB
```

## Shutdown Order

```
SIGTERM
  → grpc.GracefulStop()   — drain active RPCs
  → server.Shutdown()     — drain HTTP connections (15s)
  → batchers.Shutdown()   — flush partial batches
  → dispatcher.Shutdown() — drain in-flight messages
  → workers.Shutdown()    — drain background tasks
  → redis.Close()
  → db.Close()
  → logger.Sync()
  → exit
```
