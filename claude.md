# TJudge - Tournament System for Game Theory

## Project Overview
Tournament system for competitive programming in game theory. Programs compete against each other using tjudge-cli executor.

## Key Architecture
- **API Server**: Go + Chi router, JWT auth, WebSocket for live updates
- **Worker Pool**: Processes match queue with auto-scaling (2-10 workers)
- **tjudge-cli**: Rust-based game executor (Docker container)
- **Frontend**: React + Tailwind (embedded in Go binary)

## Important Files
- `cmd/api/main.go` - API server entry point
- `cmd/worker/main.go` - Worker entry point
- `internal/domain/tournament/service.go` - Tournament business logic
- `internal/infrastructure/executor/executor.go` - Docker execution
- `docker-compose.yml` - Local development stack

## Match Execution Flow
1. Tournament generates round-robin matches (1 per pair)
2. Matches queued in Redis (priority queue)
3. Worker dequeues and calls tjudge-cli via Docker
4. tjudge-cli runs iterations internally (`-i` parameter)
5. Results update ratings (ELO system)

## Common Commands
```bash
make docker-up          # Start all services
make migrate-up         # Apply DB migrations
make benchmark-interpret # Run benchmarks with analysis
make test               # Run tests
```

## Database
- PostgreSQL 15 with materialized views for leaderboards
- Redis for queue and caching

## CI/CD
- GitHub Actions for CI (lint, test, build)
- Docker images pushed to ghcr.io
