# Go API Template

Production-ready Go API template with clean architecture, pluggable infrastructure, and enterprise patterns.

## Quick Start

```bash
# 1. Clone & setup (interactive — pick only what you need)
git clone <repo-url> myapp && cd myapp
./scripts/init.sh

# 2. Start infrastructure
docker compose up -d

# 3. Run
make build && ./bin/go-api-template serve
```

### Setup Modes

```bash
./scripts/init.sh              # interactive — pick stacks
./scripts/init.sh --minimal    # DB + Redis only (fastest start)
./scripts/init.sh --all        # everything enabled
```

## Stack Selection

Not every project needs everything. Enable only what you use:

| Stack | What | When to enable |
|-------|------|---------------|
| **PostgreSQL** | Primary database | Always (default) |
| **Redis** | Cache, sessions, pub/sub | Most projects |
| **Kafka** | Event streaming, async processing | Microservices, event-driven |
| **Elasticsearch** | Full-text search, analytics | Search features, log aggregation |
| **Firebase** | Push notifications, mobile auth | Mobile apps |
| **SendGrid** | Transactional email | User registration, notifications |
| **Stripe** | Payment processing | E-commerce, SaaS |
| **IceWarp** | Mail server management | Enterprise email |
| **OpenTelemetry** | Distributed tracing | Microservices debugging |
| **SonarQube** | Code quality analysis | Team projects |
| **Encryption** | Field-level AES-256 | PII, GDPR compliance |

**Unconfigured services = nil = app starts fine.** No env var = service skipped.

## Project Structure

```
.
├── main.go                    # Entry point (15 lines)
├── cmd/                       # CLI commands (serve, migrate, db, cron)
├── modules/                   # Business modules
│   └── user/                  # models, types, repo, service, controller, routes, errors, events
├── pkg/                       # Shared infrastructure
│   ├── app/                   # DI container + service registry
│   ├── apperror/              # Structured errors (i18n-ready keys)
│   ├── async/                 # Worker pool + parallel execution
│   ├── auth/                  # JWT / Keycloak providers
│   ├── broker/                # Kafka producer/consumer + outbox pattern
│   ├── cache/                 # Redis cache
│   ├── circuitbreaker/        # Circuit breaker (optional)
│   ├── config/                # Viper config from .env
│   ├── cron/                  # Job scheduler
│   ├── crypto/                # AES-256 encrypt/decrypt + bcrypt + random
│   ├── database/              # Postgres + GORM + Mongo + Redis connections
│   ├── external/              # 3rd party service clients
│   │   ├── sendgrid/          #   email delivery (JSON)
│   │   ├── stripe/            #   payments (JSON)
│   │   ├── icewarp/           #   mail server (XML)
│   │   ├── firebase/          #   push notifications (gRPC)
│   │   └── elasticsearch/     #   search & analytics
│   ├── httpclient/            # Base HTTP client (TLS, retry, codec, auth)
│   ├── logger/                # Zap structured logging
│   ├── metrics/               # Prometheus metrics
│   ├── middleware/             # HTTP middleware (auth, CORS, validation, audit)
│   ├── response/              # Typed JSON responses
│   ├── retry/                 # Exponential backoff + jitter
│   ├── tracing/               # OpenTelemetry
│   └── ws/                    # WebSocket (hub, rooms, message routing)
├── db/migrations/             # SQL migrations (Goose)
├── deployments/k8s/           # Kubernetes manifests (Kustomize)
├── docker/                    # Docker configs (Prometheus, Grafana, PgBouncer)
├── scripts/                   # Setup scripts (init, sonar)
├── Dockerfile                 # Multi-stage build (non-root)
├── docker-compose.yml         # Local dev infrastructure
├── .gitlab-ci.yml             # GitLab CI/CD pipeline
└── .github/workflows/ci.yml   # GitHub Actions CI
```

## Commands

```bash
# Server
./go-api-template serve          # start API (graceful shutdown)
./go-api-template cron           # start cron scheduler

# Database
./go-api-template migrate up     # apply migrations
./go-api-template migrate status # show status
./go-api-template db test        # test connection
./go-api-template db explain "SELECT ..." # query plan
./go-api-template db slow-queries # top 20 slowest
./go-api-template db table-stats  # sizes + dead tuples
./go-api-template db locks        # who blocks who

# Make targets
make build        # build binary
make test         # run tests + coverage
make lint         # golangci-lint
make sonar-setup  # setup SonarQube
make sonar        # run analysis
```

## Adding a New Module

```bash
mkdir -p modules/order
```

Create these files (same pattern as `modules/user/`):

| File | Purpose |
|------|---------|
| `models.go` | Domain entity |
| `types.go` | Request/Response DTOs |
| `repository.go` | Interface |
| `repository_gorm.go` | GORM implementation |
| `service.go` | Business logic |
| `controller.go` | HTTP handlers |
| `routes.go` | Route registration |
| `errors.go` | `order.*` error codes |
| `events.go` | Kafka topics + event structs |

Wire in `pkg/app/app.go` → `registerModules()`.

## Adding a 3rd Party Service

```bash
mkdir -p pkg/external/twilio
```

Create 3 files:

| File | Purpose |
|------|---------|
| `client.go` | Client struct + methods |
| `types.go` | Request/Response matching 3rd party API |
| `errors.go` | ErrorDecoder + `twilio.*` error codes |

Register in `pkg/app/services.go` → `initServices()`.

## API Response Format

```json
// Success
{"status": "success", "data": {...}}
{"status": "success", "data": {"items": [...], "total": 10}}

// Error (i18n-ready keys)
{"error_code": 404, "error_message": "user.not_found", "error_detail": "User does not exist"}
```

## Key Patterns

| Pattern | Package | What |
|---------|---------|------|
| Transactional Outbox | `pkg/broker` | DB write + event in same transaction |
| Key-sharded consumers | `pkg/broker` | Parallel Kafka processing, ordered per key |
| Response audit | `pkg/middleware` | Warn on non-standard responses (debug mode) |
| Request validation | `pkg/middleware` | Validate before handler — no EOF |
| Circuit breaker | `pkg/circuitbreaker` | Stop calling dead services |
| Token forwarding | `pkg/httpclient` | Propagate user token to downstream |
| Graceful shutdown | `pkg/app` | Drain HTTP → flush batches → close resources |

## Deployment

```bash
# Docker
docker build -t myapp .
docker run -p 8080:8080 --env-file .env myapp

# Kubernetes (Kustomize)
kubectl apply -k deployments/k8s/overlays/dev
kubectl apply -k deployments/k8s/overlays/staging
kubectl apply -k deployments/k8s/overlays/prod

# CI/CD
# GitLab: .gitlab-ci.yml (lint → test → build → deploy)
# GitHub: .github/workflows/ci.yml
```

## Environment Variables

See `.env.example` for all available options, or run `./scripts/init.sh` to generate a `.env` with only the stacks you need.
