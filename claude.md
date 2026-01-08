# TJudge - Tournament System for Game Theory

## Project Overview

TJudge is a high-performance tournament system for competitive programming in game theory. Programs submit strategies that compete against each other in various games (Tic-Tac-Toe, Connect4, custom games).

**Stack:** Go 1.24, PostgreSQL 15, Redis 7, Docker, React + Tailwind

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Frontend  │────▶│   API       │────▶│   Worker    │
│   (React)   │     │   Server    │     │   Pool      │
└─────────────┘     └─────────────┘     └─────────────┘
                           │                   │
                           ▼                   ▼
                    ┌─────────────┐     ┌─────────────┐
                    │ PostgreSQL  │     │ tjudge-cli  │
                    │   + Redis   │     │  (Docker)   │
                    └─────────────┘     └─────────────┘
```

### Components

| Component | Location | Description |
|-----------|----------|-------------|
| API Server | `cmd/api/` | REST API + WebSocket, JWT auth |
| Worker | `cmd/worker/` | Match processing, auto-scaling pool |
| Frontend | `web/` | React SPA, embedded in Go binary |
| tjudge-cli | External | Rust game executor (Docker image) |

## Key Files

### Entry Points
- `cmd/api/main.go` - API server startup, dependency injection
- `cmd/worker/main.go` - Worker pool startup
- `cmd/migrations/main.go` - Database migrations runner
- `cmd/benchmark/main.go` - Benchmark interpreter

### Domain Logic
- `internal/domain/models.go` - Core domain entities (Tournament, Match, Program, User)
- `internal/domain/tournament/service.go` - Tournament management, round-robin generation
- `internal/domain/auth/service.go` - Authentication service
- `internal/domain/rating/elo.go` - ELO rating calculations

### Infrastructure
- `internal/infrastructure/db/` - PostgreSQL repositories
- `internal/infrastructure/cache/` - Redis caching layer
- `internal/infrastructure/queue/` - Match queue (Redis)
- `internal/infrastructure/executor/executor.go` - Docker-based match executor
- `internal/worker/pool.go` - Worker pool with auto-scaling

### API
- `internal/api/routes.go` - Route definitions
- `internal/api/handlers/` - HTTP handlers
- `internal/api/middleware/` - Auth, rate limiting, logging

### Frontend
- `web/src/` - React application source
- `internal/web/embed.go` - Frontend embedding for single binary

## Database Schema

### Core Tables
```sql
users           -- User accounts (username, email, password_hash, role)
programs        -- Submitted programs (user_id, code, language, status)
tournaments     -- Tournament definitions (name, game_type, status)
matches         -- Match records (tournament_id, program1_id, program2_id, result)
ratings         -- ELO ratings per tournament (program_id, tournament_id, rating)
```

### Materialized Views
```sql
leaderboard_global      -- Global rankings across all tournaments
leaderboard_tournament  -- Per-tournament rankings
```

Migrations: `migrations/000001_*.sql` through `migrations/000016_*.sql`

## Match Execution Flow

1. **Tournament Creation**: Admin creates tournament with game type
2. **Program Submission**: Users upload programs
3. **Match Generation**: Round-robin pairs (1 match per pair)
4. **Queue**: Matches added to Redis priority queue
5. **Worker Processing**:
   - Worker dequeues match
   - Spawns Docker container with tjudge-cli
   - Passes programs via volume mount
   - tjudge-cli runs iterations internally (`-i` parameter)
6. **Result Processing**: ELO ratings updated, WebSocket broadcast

```go
// Match generation (simplified from tournament/service.go)
for i := 0; i < len(participants); i++ {
    for j := i + 1; j < len(participants); j++ {
        // 1 match per pair, iterations inside tjudge-cli
        createMatch(participants[i], participants[j])
    }
}
```

## Configuration

### Environment Variables
```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=tjudge
DB_PASSWORD=secret
DB_NAME=tjudge

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# JWT
JWT_SECRET=your-32-char-secret-minimum

# Worker
WORKER_MIN=2
WORKER_MAX=10

# Server
SERVER_PORT=8080
LOG_LEVEL=info
```

## Development Commands

```bash
# Setup
make docker-up          # Start PostgreSQL, Redis
make migrate-up         # Apply migrations

# Development
make run-api            # Start API server
make run-worker         # Start worker

# Testing
make test               # Run unit tests
make test-race          # Run with race detector
make benchmark          # Run performance benchmarks
make benchmark-interpret # Run benchmarks with analysis

# Building
make build              # Build all binaries
make docker-build       # Build Docker images

# Utilities
make lint               # Run golangci-lint
make admin EMAIL=x@y.z  # Make user admin
```

## API Endpoints

### Auth
- `POST /api/v1/auth/register` - User registration
- `POST /api/v1/auth/login` - Login (returns JWT)
- `POST /api/v1/auth/refresh` - Refresh token
- `GET /api/v1/auth/me` - Current user

### Tournaments
- `GET /api/v1/tournaments` - List tournaments
- `POST /api/v1/tournaments` - Create tournament (admin)
- `GET /api/v1/tournaments/:id` - Get tournament
- `POST /api/v1/tournaments/:id/join` - Join tournament
- `GET /api/v1/tournaments/:id/leaderboard` - Get leaderboard

### Programs
- `POST /api/v1/programs` - Upload program
- `GET /api/v1/programs` - List user's programs
- `GET /api/v1/programs/:id` - Get program details

### WebSocket
- `WS /api/v1/ws/tournaments/:id` - Real-time updates (leaderboard, match results)

## Testing

### Unit Tests
Located alongside source files: `*_test.go`

### Benchmark Tests
`tests/benchmark/` - Performance benchmarks with expected standards

```bash
make benchmark-interpret  # Run with interpretation
go run ./cmd/benchmark -standards  # Show expected values
```

### Integration/E2E Tests
`tests/integration/`, `tests/e2e/`

## Performance Standards

| Operation | Expected | Category |
|-----------|----------|----------|
| Health Endpoint | 50µs | API |
| Tournament List | 5ms | API |
| Leaderboard | 10ms | API |
| Queue Enqueue | 500µs | Queue |
| Match Create (DB) | 2ms | DB |
| Worker Pool (100 matches) | 100ms | Worker |

Run `make benchmark-interpret` for full analysis.

## Docker

### Images
- `tjudge-api` - API server
- `tjudge-worker` - Worker service
- `tjudge-cli` - Game executor (Rust)

### docker-compose.yml Services
- `api` - API server
- `worker` - Worker pool
- `postgres` - Database
- `redis` - Cache + Queue
- `prometheus` - Metrics
- `grafana` - Dashboards

## CI/CD

### GitHub Actions
- `.github/workflows/ci.yml` - Lint, Test, Build, Security scan
- `.github/workflows/cd.yml` - Build and push Docker images

### Deployment
Docker Compose based deployment with blue-green scripts in `scripts/`.

## Common Issues

### "package requires newer Go version"
Ensure Go 1.24+ is installed. Check `go version`.

### "pattern all:dist: no matching files found"
Frontend not built. Run `cd web && npm run build` or use `make docker-build`.

### "resource temporarily unavailable" in worker
Reduce worker count: `WORKER_MIN=2 WORKER_MAX=5`

### Database connection issues
Check `DB_HOST`, `DB_PORT`, `DB_PASSWORD` in `.env`.

## File Structure

```
TJudge/
├── cmd/                    # Entry points
│   ├── api/                # API server
│   ├── worker/             # Worker service
│   ├── migrations/         # Migration tool
│   └── benchmark/          # Benchmark interpreter
├── internal/
│   ├── api/                # HTTP layer
│   │   ├── handlers/       # Request handlers
│   │   ├── middleware/     # Middleware
│   │   └── routes.go       # Route definitions
│   ├── domain/             # Business logic
│   │   ├── auth/           # Authentication
│   │   ├── tournament/     # Tournament service
│   │   ├── rating/         # ELO calculations
│   │   └── models.go       # Domain entities
│   ├── infrastructure/     # External services
│   │   ├── db/             # PostgreSQL
│   │   ├── cache/          # Redis
│   │   ├── queue/          # Match queue
│   │   └── executor/       # Docker executor
│   ├── worker/             # Worker pool
│   ├── websocket/          # Real-time updates
│   └── web/                # Embedded frontend
├── web/                    # React frontend
├── migrations/             # SQL migrations
├── docker/                 # Dockerfiles
├── tests/                  # Test suites
│   ├── benchmark/          # Performance tests
│   ├── integration/        # Integration tests
│   └── e2e/                # End-to-end tests
├── scripts/                # Deployment scripts
├── docs/                   # Documentation
├── docker-compose.yml      # Local development
└── Makefile                # Build commands
```

## Useful Queries

```sql
-- Check pending matches
SELECT COUNT(*) FROM matches WHERE status = 'pending';

-- Tournament leaderboard
SELECT * FROM leaderboard_tournament WHERE tournament_id = 'uuid';

-- Refresh materialized views
REFRESH MATERIALIZED VIEW CONCURRENTLY leaderboard_global;

-- User's programs with ratings
SELECT p.name, r.rating, r.wins, r.losses
FROM programs p
JOIN ratings r ON p.id = r.program_id
WHERE p.user_id = 'uuid';
```

## Monitoring

- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (admin/admin)
- API Metrics: `http://localhost:8080/metrics`

## Documentation

| Document | Description |
|----------|-------------|
| `docs/SETUP.md` | Development setup, deployment, configuration |
| `docs/USER_GUIDE.md` | User guide, game rules, strategy examples |
| `docs/ARCHITECTURE.md` | Detailed architecture |
| `docs/API_GUIDE.md` | Full API reference |
| `docs/DATABASE_SCHEMA.md` | Database schema |

## Links

- API Server: `http://localhost:8080`
- WebSocket: `ws://localhost:8080/api/v1/ws/tournaments/:id`
- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
