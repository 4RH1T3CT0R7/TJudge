.PHONY: help build test lint run-api run-worker docker-build docker-build-executor docker-up docker-down migrate-up migrate-down clean admin benchmark benchmark-interpret test-load deploy deploy-weak deploy-medium deploy-strong detect-profile backup restore backup-list

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
	@echo ""
	@echo "  make clean         - Clean build artifacts"

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
	golangci-lint run --timeout=5m

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

# Build only tjudge-cli executor image
docker-build-executor:
	@echo "Building tjudge-cli executor image..."
	docker build -t tjudge-cli:latest -f docker/tjudge/Dockerfile .

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

# Generate mocks
mocks:
	@echo "Generating mocks..."
	@which mockgen > /dev/null || (echo "Installing mockgen..." && go install github.com/golang/mock/mockgen@latest)
	go generate ./...

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

# Make user admin by email
admin:
ifndef EMAIL
	@echo "Usage: make admin EMAIL=user@example.com"
	@exit 1
endif
	@echo "Making $(EMAIL) an admin..."
	@docker exec tjudge-postgres psql -U tjudge -d tjudge -c \
		"UPDATE users SET role = 'admin' WHERE email = '$(EMAIL)' RETURNING username, email, role;" \
		|| echo "Failed to update user. Make sure the container is running and user exists."
	@echo ""
	@echo "Done! User must log out and log in again to get the new role."

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
