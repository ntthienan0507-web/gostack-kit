.PHONY: dev build sqlc lint test mock migrate-create

BINARY := bin/server
MIGRATIONS_DIR := db/migrations

# Development
dev:
	go run ./cmd/server/main.go

# Build
build:
	go build -o $(BINARY) ./cmd/server/main.go

# SQLC code generation
sqlc:
	sqlc generate

# Linting
lint:
	golangci-lint run ./...

# Testing
test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Create migration file
migrate-create:
	@test -n "$(NAME)" || (echo "Usage: make migrate-create NAME=create_posts"; exit 1)
	goose -dir $(MIGRATIONS_DIR) create $(NAME) sql

# Mock generation (requires mockgen)
mock:
	@mkdir -p internal/module/user/mock
	mockgen -source=internal/module/user/repository.go \
		-destination=internal/module/user/mock/repository_mock.go \
		-package=mock

# Format
fmt:
	gofmt -w .

# Vet
vet:
	go vet ./...

# Clean
clean:
	rm -rf bin/ coverage.out
