# TODO

## High Priority

- [x] **Outbox relay integration** — relay started in `app.go` `Run()` as background goroutine when Kafka producer + GORM are available. Stops via context cancellation on shutdown.
- [x] **Slim down template** — resolved by `create-go-api` tool which clones gostack-kit at runtime and includes only selected stacks. No more bloated go.mod.
- [x] **Graceful shutdown Kafka consumer** — `Close()` does: cancel context → wait for in-flight messages → flush batchers → drain dispatchers → close consumer group. Shutdown order in `app.go`: consumer → workers → producer → Redis → DB.

## Medium Priority

- [x] **Error propagation tests** — `modules/user/error_propagation_test.go`: 16 tests covering pgx.ErrNoRows, pgconn violations, AppError passthrough, wrapped errors → correct HTTP codes.
- [x] **Distributed rate limit** — `pkg/middleware/ratelimit_redis.go`: Redis sliding window via Lua script. Key functions: `KeyByIP()`, `KeyByUserID()`, `KeyByEndpoint()`, `KeyByUserAndEndpoint()`. Fail-open on Redis errors.
- [x] **Swagger CI check** — `swagger` job in both GitHub Actions and GitLab CI. Regenerates docs, fails if diff detected.
- [x] **Docker Compose diet** — profiles: `observability` (jaeger, prometheus, grafana), `sonar` (sonarqube), `all`. Core (postgres, redis, app) runs without profiles.

## Low Priority

- [x] **Auth e2e test** — `modules/user/auth_e2e_test.go`: no token → 401, invalid token → 401, role-based access, full CRUD flow, token refresh.
- [x] **GORM integration test** — `modules/user/integration_gorm_test.go`: same coverage as SQLC tests using `testutil.NewGormDB(t)`.
