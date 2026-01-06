# TJudge Development Guide

## Quick Start

### 1. Prerequisites

- Go 1.24+
- Docker & Docker Compose
- Make

### 2. Start Development Environment

```bash
# Start database and cache
docker compose up -d postgres redis

# Run migrations
make migrate-up

# Start API server with hot reload
make dev
```

Server will be available at: http://localhost:8080

### 3. Verify Installation

```bash
curl http://localhost:8080/health
# Output: OK
```

---

## Make Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start API with hot reload (air) |
| `make build` | Build all binaries |
| `make test` | Run unit tests |
| `make test-integration` | Run integration tests |
| `make test-e2e` | Run E2E tests |
| `make lint` | Run linter (golangci-lint) |
| `make migrate-up` | Apply database migrations |
| `make migrate-down` | Rollback migrations |
| `make docker-up` | Start all services in Docker |
| `make docker-down` | Stop all Docker services |
| `make clean` | Remove build artifacts |

---

## API Endpoints

### Health Check
```
GET /health
```

### Authentication

```bash
# Register new user
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "player1",
  "email": "player1@example.com",
  "password": "SecurePass123!"
}

# Response:
{
  "access_token": "eyJ...",
  "refresh_token": "eyJ...",
  "user": {
    "id": "uuid",
    "username": "player1",
    "email": "player1@example.com"
  }
}
```

```bash
# Login
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "player1",
  "password": "SecurePass123!"
}
```

```bash
# Refresh token
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJ..."
}
```

```bash
# Get current user (requires auth)
GET /api/v1/auth/me
Authorization: Bearer <access_token>
```

```bash
# Logout
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
```

### Tournaments

```bash
# List tournaments (public)
GET /api/v1/tournaments
GET /api/v1/tournaments?game_type=tictactoe&status=active&limit=10

# Get tournament by ID (public)
GET /api/v1/tournaments/{id}

# Get leaderboard (public)
GET /api/v1/tournaments/{id}/leaderboard

# Get tournament matches (public)
GET /api/v1/tournaments/{id}/matches
```

```bash
# Create tournament (auth required)
POST /api/v1/tournaments
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "name": "My Tournament",
  "description": "Tournament description",
  "game_type": "tictactoe",
  "max_participants": 16
}
```

```bash
# Join tournament (auth required)
POST /api/v1/tournaments/{id}/join
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "program_id": "uuid-of-your-program"
}
```

```bash
# Start tournament (auth required, organizer only)
POST /api/v1/tournaments/{id}/start
Authorization: Bearer <access_token>
```

### Programs

All program endpoints require authentication.

```bash
# Create program
POST /api/v1/programs
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "name": "My Bot",
  "code_path": "path/to/code",
  "language": "python",
  "game_type": "tictactoe"
}
```

```bash
# List your programs
GET /api/v1/programs
Authorization: Bearer <access_token>

# Get program by ID
GET /api/v1/programs/{id}
Authorization: Bearer <access_token>

# Update program
PUT /api/v1/programs/{id}
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "name": "Updated Bot Name",
  "code_path": "new/path"
}

# Delete program
DELETE /api/v1/programs/{id}
Authorization: Bearer <access_token>
```

### Matches

```bash
# List matches
GET /api/v1/matches
GET /api/v1/matches?tournament_id={id}&status=completed

# Get match by ID
GET /api/v1/matches/{id}

# Get statistics
GET /api/v1/matches/statistics
```

### WebSocket (Real-time updates)

```javascript
// Connect to tournament updates
const ws = new WebSocket('ws://localhost:8080/api/v1/ws/tournaments/{tournament_id}');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Update:', data);
};
```

---

## Configuration

Configuration is loaded from environment variables. Create `.env` file:

```env
# Environment
ENVIRONMENT=development

# API Server
API_PORT=8080
READ_TIMEOUT=30s
WRITE_TIMEOUT=30s

# Database (PostgreSQL)
DB_HOST=localhost
DB_PORT=5433          # Note: 5433 to avoid conflict with local PostgreSQL
DB_USER=tjudge
DB_PASSWORD=secret
DB_NAME=tjudge
DB_MAX_CONNECTIONS=10

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT
JWT_SECRET=your-secret-key-change-in-production
JWT_ACCESS_TTL=1h
JWT_REFRESH_TTL=168h

# Logging
LOG_LEVEL=debug       # debug, info, warn, error
LOG_FORMAT=console    # console, json

# Rate Limiting
RATE_LIMIT_ENABLED=false
```

---

## Common Issues & Solutions

### 1. "role tjudge does not exist"

**Problem:** Local PostgreSQL is running on port 5432, conflicting with Docker.

**Solution:**
```bash
# Check what's on port 5432
lsof -i :5432

# If local postgres is running, use port 5433 for Docker
# In docker-compose.yml:
ports:
  - "5433:5432"

# In .env:
DB_PORT=5433
```

### 2. "air: command not found"

**Problem:** Air is not in PATH.

**Solution:**
```bash
# Air is installed in ~/go/bin
export PATH=$PATH:~/go/bin

# Or add to .zshrc / .bashrc
echo 'export PATH=$PATH:~/go/bin' >> ~/.zshrc
```

### 3. Migrations fail

**Problem:** Database not running or wrong credentials.

**Solution:**
```bash
# Check if postgres is running
docker compose ps

# Check connection
docker exec tjudge-postgres pg_isready -U tjudge

# Restart containers
docker compose down -v
docker compose up -d postgres redis
```

### 4. "connection refused" on localhost:8080

**Problem:** Server not running or crashed.

**Solution:**
```bash
# Check if something else uses port 8080
lsof -i :8080

# Check server logs in terminal where make dev is running
# Look for FATAL or ERROR messages
```

### 5. "Internal server error" on API calls

**Problem:** Usually database tables missing.

**Solution:**
```bash
# Run migrations
make migrate-up

# Check migration status
docker exec tjudge-postgres psql -U tjudge -d tjudge -c "\dt"
```

### 6. Docker containers unhealthy

**Problem:** Service failed to start properly.

**Solution:**
```bash
# Check logs
docker compose logs postgres
docker compose logs redis

# Full reset
docker compose down -v
docker compose up -d postgres redis
```

---

## Project Structure

```
TJudge/
├── cmd/
│   ├── api/            # API server entry point
│   ├── worker/         # Background worker entry point
│   └── migrations/     # Migration tool
├── internal/
│   ├── api/            # HTTP handlers, routes, middleware
│   ├── config/         # Configuration loading
│   ├── domain/         # Business logic (auth, tournament, rating)
│   ├── infrastructure/ # DB, cache, queue implementations
│   ├── websocket/      # WebSocket hub and clients
│   └── worker/         # Background job processing
├── pkg/                # Shared packages (logger, errors, metrics)
├── migrations/         # SQL migration files
├── tests/
│   ├── integration/    # Integration tests
│   ├── e2e/            # End-to-end tests
│   └── chaos/          # Chaos/stress tests
├── docker/             # Dockerfiles
├── deployments/        # Kubernetes, Prometheus configs
└── docs/               # Documentation
```

---

## Testing

```bash
# Unit tests
make test

# With coverage
make test-coverage

# Integration tests (requires running DB)
RUN_INTEGRATION=true make test-integration

# E2E tests (requires running server)
make test-e2e

# Linting
make lint
```

---

## Metrics & Monitoring

Metrics endpoint: http://localhost:9090/metrics

Available metrics:
- `tjudge_http_requests_total` - HTTP request count
- `tjudge_http_request_duration_seconds` - Request latency
- `tjudge_matches_total` - Processed matches
- `tjudge_match_duration_seconds` - Match execution time
- `tjudge_queue_size` - Queue size by priority
- `tjudge_active_workers` - Active worker count
- `tjudge_cache_hits_total` / `tjudge_cache_misses_total` - Cache statistics

---

## Full Docker Deployment

To run everything in Docker:

```bash
# Start all services
docker compose up -d

# Services:
# - postgres (5433)
# - redis (6379)
# - api (8080)
# - worker
# - prometheus (9090)
# - grafana (3000)
```

Grafana: http://localhost:3000 (admin/admin)
