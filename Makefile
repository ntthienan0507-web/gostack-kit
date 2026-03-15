.PHONY: dev build sqlc swagger lint test test-integration fmt vet clean help \
	migrate migrate-status migrate-rollback migrate-create \
	mock test-db db-tables db-shell

APP     := go-api-template
BINARY  := bin/$(APP)

# ============================================
# Development
# ============================================

dev: build ## Build and start the server
	./$(BINARY) serve

build: ## Build binary
	go build -o $(BINARY) ./cmd/$(APP)

run: ## Run without building (development)
	go run ./cmd/$(APP) serve

test: ## Run tests with coverage
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

test-integration: ## Run integration tests (requires Docker)
	go test -tags integration -race -count=1 -timeout 120s ./...

lint: ## Run linter
	golangci-lint run ./...

fmt: ## Format code
	gofmt -w .

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out docs/

# ============================================
# Database
# ============================================

migrate: ## Run pending migrations
	./$(BINARY) migrate up

migrate-status: ## Show migration status
	./$(BINARY) migrate status

migrate-rollback: ## Rollback last migration
	./$(BINARY) migrate down

migrate-create: ## Create migration (NAME=create_posts)
	@test -n "$(NAME)" || (echo "Usage: make migrate-create NAME=create_posts"; exit 1)
	./$(BINARY) migrate create $(NAME)

test-db: ## Test database connection
	./$(BINARY) db test

db-tables: ## List all tables
	./$(BINARY) db tables

db-shell: ## Open psql shell
	./$(BINARY) db shell

# ============================================
# Code Generation
# ============================================

sqlc: ## Generate SQLC code from queries
	sqlc generate

swagger: ## Generate Swagger docs from annotations
	swag init -g cmd/$(APP)/main.go --output docs

mock: ## Generate mocks (requires mockgen)
	@mkdir -p internal/module/user/mock
	mockgen -source=internal/module/user/repository.go \
		-destination=internal/module/user/mock/repository_mock.go \
		-package=mock

# ============================================
# Help
# ============================================

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "CLI: $(APP) <command>"
	@echo "  serve                        Start the API server"
	@echo "  migrate up|status|down|create|up-to|down-to|version|fix"
	@echo "  db      test|tables|columns|query|shell"
