.PHONY: status help build test lint run-api run-worker docker-build docker-build-executor docker-up docker-down migrate-up migrate-down clean admin create-user benchmark benchmark-interpret test-load test-contract generate-contract-mocks deploy deploy-weak deploy-medium deploy-strong detect-profile backup restore backup-list generate tools

# Default target
help:
	@echo "TJudge - High-Load Tournament System"
	@echo ""
	@echo "Available targets:"
	@echo ""
	@echo "  === Quick Deploy (Self-Hosted) ==="
	@echo "  make deploy        - Auto-detect profile and deploy"
	@echo "  make deploy-weak   - Deploy for weak hardware (2 cores, 4GB RAM)"
	@echo "  make deploy-medium - Deploy for medium hardware (4 cores, 8GB RAM)"
	@echo "  make deploy-strong - Deploy for strong hardware (8+ cores, 16GB+ RAM)"
	@echo "  make detect-profile - Detect recommended profile for your hardware"
	@echo ""
	@echo "  === Backup & Restore ==="
	@echo "  make backup        - Create database backup"
	@echo "  make restore       - Restore from backup (BACKUP=path/to/file.sql.gz)"
	@echo "  make backup-list   - List available backups"
	@echo ""
	@echo "  === Development ==="
	@echo "  make deps          - Download dependencies"
	@echo "  make build         - Build all binaries"
	@echo "  make run-api       - Run API server locally"
	@echo "  make run-worker    - Run worker locally"
	@echo "  make lint          - Run linters"
	@echo "  make fmt           - Format code"
	@echo ""
	@echo "  === Testing ==="
	@echo "  make test          - Run all tests"
	@echo "  make test-race     - Run tests with race detector"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make test-contract - Run contract/API tests (no external services)"
	@echo "  make test-e2e      - Run end-to-end tests"
	@echo "  make benchmark     - Run performance benchmarks"
	@echo "  make test-load     - Run load tests"
	@echo ""
	@echo "  === Docker ==="
	@echo "  make docker-build  - Build all Docker images"
	@echo "  make docker-up     - Start Docker Compose (dev)"
	@echo "  make docker-down   - Stop Docker Compose"
	@echo ""
	@echo "  === Database ==="
	@echo "  make migrate-up    - Apply database migrations"
	@echo "  make migrate-down  - Rollback database migrations"
	@echo "  make admin         - Make user admin (EMAIL=user@example.com)"
	@echo "  make create-user   - Register user via API (EMAIL=, USERNAME=, PASSWORD= [ADMIN=1])"
	@echo ""
	@echo "  make clean         - Clean build artifacts"

# Install development tools
tools:
	@echo "Installing development tools..."
	go install go.uber.org/mock/mockgen@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate mocks, swagger, and other code
generate:
	@echo "Generating code..."
	go generate ./...
	@which swag > /dev/null 2>&1 && swag init -g cmd/api/main.go -o docs/swagger --parseInternal --quiet || true
	@echo "Done."

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/migrate ./cmd/migrations

# Run tests
test:
	@echo "Running tests..."
	go test -v -count=1 ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -race -v -count=1 ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./... --timeout=5m

# Run API server
run-api:
	@echo "Starting API server..."
	go run ./cmd/api

# Run worker
run-worker:
	@echo "Starting worker..."
	go run ./cmd/worker

# Build Docker images
docker-build:
	@echo "Building Docker images..."
	docker build -t tjudge/api:latest -f docker/api/Dockerfile .
	docker build -t tjudge/worker:latest -f docker/worker/Dockerfile .
	docker build -t tjudge-cli:latest -f docker/tjudge/Dockerfile .
	docker build -t tjudge-builder:latest -f docker/builder/Dockerfile .

# Build only tjudge-cli executor image
docker-build-executor:
	@echo "Building tjudge-cli executor image..."
	docker build -t tjudge-cli:latest -f docker/tjudge/Dockerfile .

# Show full system status (containers, images, health, /system/status)
status:
	@./scripts/status.sh

# Build only tjudge-builder compile sandbox image
docker-build-builder:
	@echo "Building tjudge-builder compile sandbox image..."
	docker build -t tjudge-builder:latest -f docker/builder/Dockerfile .

# Start Docker Compose
docker-up:
	@echo "Starting Docker Compose..."
	docker-compose up -d
	@echo "Services started. Waiting for health checks..."
	@sleep 5
	@docker-compose ps

# Stop Docker Compose
docker-down:
	@echo "Stopping Docker Compose..."
	docker-compose down

# View Docker logs
docker-logs:
	docker-compose logs -f

# Apply database migrations
migrate-up:
	@echo "Applying database migrations..."
	go run ./cmd/migrations up

# Rollback database migrations
migrate-down:
	@echo "Rolling back database migrations..."
	go run ./cmd/migrations down

# Create new migration
migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Development mode (run with hot reload using air)
dev:
	@which air > /dev/null 2>&1 || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	@$(shell go env GOPATH)/bin/air

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Generate mocks (alias for generate)
mocks: generate

# Generate mocks for contract tests
generate-contract-mocks:
	@echo "Generating contract test mocks..."
	@which mockery > /dev/null 2>&1 || (echo "Installing mockery..." && go install github.com/vektra/mockery/v2@latest)
	mockery
	@echo "Done."

# Run contract/API tests (no external services needed)
test-contract:
	@echo "Running contract tests..."
	go test -tags=contract -count=1 -parallel=8 -timeout=5m ./tests/contract/...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./tests/integration/...

# Run E2E tests
test-e2e:
	@echo "Running E2E tests..."
	go test -v -tags=e2e ./tests/e2e/...

# Run performance benchmarks
benchmark:
	@echo "Running performance benchmarks..."
	@echo "Note: Benchmarks requiring DB/Redis will be skipped if services are not running"
	@echo ""
	go test -tags=benchmark -bench=. -benchmem -benchtime=1s ./tests/benchmark/...

# Run benchmarks with interpretation
benchmark-interpret:
	@echo "Running benchmarks with interpretation..."
	@go run ./cmd/benchmark -run

# Run benchmark with specific pattern
benchmark-api:
	@echo "Running API benchmarks..."
	go test -tags=benchmark -bench=BenchmarkHealth -benchmem ./tests/benchmark/...

benchmark-worker:
	@echo "Running Worker benchmarks..."
	go test -tags=benchmark -bench=BenchmarkWorkerPool -benchmem ./tests/benchmark/...

benchmark-queue:
	@echo "Running Queue benchmarks..."
	go test -tags=benchmark -bench=BenchmarkQueue -benchmem ./tests/benchmark/...

benchmark-db:
	@echo "Running Database benchmarks..."
	go test -tags=benchmark -bench=BenchmarkDB -benchmem ./tests/benchmark/...

# Load testing
test-load:
	@echo "Running load tests..."
	@echo "Make sure the API server is running on localhost:8080"
	@echo ""
	go test -tags=load -v -timeout=5m ./tests/load/...

# Quick load test (shorter duration)
test-load-quick:
	@echo "Running quick load tests..."
	go test -tags=load -v -short -timeout=2m ./tests/load/...

# Security scan
security:
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec ./...
	@which govulncheck > /dev/null || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	govulncheck ./...

# Make user admin by email. Auto-detects postgres container (local or prod).
admin:
ifndef EMAIL
	@echo "Usage: make admin EMAIL=user@example.com"
	@exit 1
endif
	@set -e; \
	container=$$(docker ps --filter "name=tjudge-postgres" --format '{{.Names}}' | head -1); \
	if [ -z "$$container" ]; then \
		echo "Error: no running tjudge-postgres* container found"; \
		exit 1; \
	fi; \
	echo "Promoting $(EMAIL) to admin via container $$container..."; \
	result=$$(docker exec "$$container" psql -U tjudge -d tjudge -tA -c \
		"UPDATE users SET role = 'admin' WHERE email = '$(EMAIL)' RETURNING username, email, role;"); \
	if [ -z "$$result" ]; then \
		echo "Error: no user with email '$(EMAIL)' (UPDATE matched 0 rows)"; \
		exit 1; \
	fi; \
	echo "Updated: $$result"; \
	echo "Done! User must log out and log in again to get the new role."

# Create user via API (registers a new account). Pass ADMIN=1 to promote to admin.
# Usage:
#   make create-user EMAIL=a@b.c USERNAME=alice PASSWORD=secret123
#   make create-user EMAIL=a@b.c USERNAME=alice PASSWORD=secret123 ADMIN=1
create-user:
ifndef EMAIL
	@echo "Usage: make create-user EMAIL=user@example.com USERNAME=user PASSWORD=secret [ADMIN=1]"
	@exit 1
endif
ifndef USERNAME
	@echo "Usage: make create-user EMAIL=user@example.com USERNAME=user PASSWORD=secret [ADMIN=1]"
	@exit 1
endif
ifndef PASSWORD
	@echo "Usage: make create-user EMAIL=user@example.com USERNAME=user PASSWORD=secret [ADMIN=1]"
	@exit 1
endif
	@API_URL=$${API_URL:-http://localhost:8080}; \
	echo "Registering $(USERNAME) <$(EMAIL)> at $$API_URL..."; \
	HTTP_CODE=$$(curl -sS -o /tmp/tjudge-create-user.json -w "%{http_code}" \
		-X POST "$$API_URL/api/v1/auth/register" \
		-H 'Content-Type: application/json' \
		-d '{"username":"$(USERNAME)","email":"$(EMAIL)","password":"$(PASSWORD)"}'); \
	echo "HTTP $$HTTP_CODE"; \
	cat /tmp/tjudge-create-user.json; echo; \
	if [ "$$HTTP_CODE" != "200" ] && [ "$$HTTP_CODE" != "201" ]; then \
		echo "Registration failed."; rm -f /tmp/tjudge-create-user.json; exit 1; \
	fi; \
	rm -f /tmp/tjudge-create-user.json
ifeq ($(ADMIN),1)
	@$(MAKE) admin EMAIL=$(EMAIL)
endif

# =============================================================================
# Self-Hosted Deployment
# =============================================================================

# Auto-detect profile and deploy
deploy:
	@./scripts/quick-deploy.sh

# Deploy with weak profile (2 cores, 4GB RAM)
deploy-weak:
	@./scripts/quick-deploy.sh weak

# Deploy with medium profile (4 cores, 8GB RAM)
deploy-medium:
	@./scripts/quick-deploy.sh medium

# Deploy with strong profile (8+ cores, 16GB+ RAM)
deploy-strong:
	@./scripts/quick-deploy.sh strong

# Detect recommended profile for your hardware
detect-profile:
	@./scripts/detect-profile.sh

# =============================================================================
# Backup & Restore
# =============================================================================

# Create database backup
backup:
	@./scripts/backup.sh

# Restore database from backup
restore:
ifndef BACKUP
	@echo "Usage: make restore BACKUP=backups/tjudge_YYYYMMDD_HHMMSS.sql.gz"
	@echo ""
	@echo "Available backups:"
	@ls -lh backups/tjudge_*.sql.gz 2>/dev/null || echo "  No backups found"
	@exit 1
endif
	@./scripts/restore.sh $(BACKUP)

# List available backups
backup-list:
	@echo "Available backups:"
	@ls -lh backups/tjudge_*.sql.gz 2>/dev/null || echo "  No backups found"
