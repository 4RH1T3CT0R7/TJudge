# Тестирование производительности TJudge

Данный документ описывает инструменты для тестирования производительности системы TJudge.

## Быстрый старт

```bash
# Запустить все бенчмарки
make benchmark

# Запустить бенчмарки с интерпретацией
make benchmark-interpret

# Запустить нагрузочные тесты (требуется работающий API)
make test-load
```

---

## Типы тестов

### 1. Бенчмарки (Benchmarks)

Бенчмарки измеряют производительность отдельных компонентов системы.

#### Запуск

```bash
# Все бенчмарки
make benchmark

# Только API бенчмарки
make benchmark-api

# Только Worker бенчмарки
make benchmark-worker

# Только Queue бенчмарки
make benchmark-queue

# Только Database бенчмарки
make benchmark-db

# С интерпретацией результатов
make benchmark-interpret
```

#### Что тестируется

| Категория | Тесты |
|-----------|-------|
| **API** | Health endpoint, Auth login, Tournament list, Leaderboard |
| **Worker** | Throughput (small/medium/large pools), Processing latency, Autoscaling |
| **Queue** | Enqueue, Dequeue, Priority operations |
| **Database** | Health check, User lookup, Tournament list, Match creation |

#### Пример вывода

```
BenchmarkHealthEndpoint-8       50000         23450 ns/op        1024 B/op       12 allocs/op
BenchmarkTournamentsList-8      10000        145230 ns/op        8192 B/op       89 allocs/op
BenchmarkWorkerPool_ThroughputMedium-8    100     15234567 ns/op    102400 B/op    1523 allocs/op
```

---

### 2. Нагрузочные тесты (Load Tests)

Нагрузочные тесты проверяют поведение системы под высокой нагрузкой.

#### Запуск

```bash
# Полные нагрузочные тесты
make test-load

# Быстрые тесты
make test-load-quick

# С кастомным URL
LOAD_API_URL=http://localhost:8080 go test -tags=load -v ./tests/load/...
```

#### Сценарии тестирования

| Тест | Concurrency | Duration | Описание |
|------|-------------|----------|----------|
| Health Endpoint | 50 | 10s | Базовая проверка throughput |
| Tournaments List | 30 | 10s | Тестирование API listing |
| Auth Login | 20 | 10s | Тестирование аутентификации |
| Mixed Endpoints | 40 | 15s | Комбинированная нагрузка |
| Rate Limiting | 100 | 5s | Проверка rate limiter |
| Sustained Load | 25 | 30s | Длительная нагрузка |
| Burst Traffic | 5x200 | — | Всплески трафика |

#### Пример вывода

```
=== Health Endpoint Load Test Results ===
Total Requests:     125000
Successful:         124875 (99.90%)
Failed:             125
Avg Latency:        3.45 ms
Min Latency:        1 ms
Max Latency:        156 ms
Requests/sec:       12500.00
================================
```

---

### 3. Performance тесты

Тесты производительности для специфических сценариев.

#### Запуск

```bash
# Тест турнира с 30 командами
go test -v ./tests/performance/... -run Tournament30Teams

# Тест загрузки программ
go test -v ./tests/performance/... -run Upload30Teams
```

#### Сценарии

| Тест | Описание |
|------|----------|
| Tournament30Teams | Полный цикл турнира с 30 командами |
| Upload30Teams | Загрузка программ от 30 команд |
| UploadTugOfWar | Специфический тест для игры Tug of War |

---

### 4. Хаос-тесты (Chaos Tests)

Тесты устойчивости системы к сбоям.

#### Запуск

```bash
go test -v ./tests/chaos/...
```

#### Сценарии

- Отключение БД
- Отключение Redis
- Перезапуск воркеров
- Высокая нагрузка + сбои

---

## Метрики производительности

Worker и API экспортируют метрики в формате Prometheus.

### Доступные метрики

```
# Match метрики
tjudge_matches_total{status, game_type}
tjudge_match_duration_seconds{game_type}
tjudge_matches_in_progress

# Queue метрики
tjudge_queue_size{priority}
tjudge_queue_wait_time_seconds{priority}

# Worker метрики
tjudge_active_workers
tjudge_worker_pool_size

# HTTP метрики
tjudge_http_requests_total{method, path, status}
tjudge_http_request_duration_seconds{method, path}

# Database метрики
tjudge_db_query_duration_seconds{query_type}
tjudge_db_connections{state}

# Cache метрики
tjudge_cache_hits_total{cache_type}
tjudge_cache_misses_total{cache_type}
```

### Просмотр метрик

```bash
# API метрики
curl http://localhost:8080/metrics

# Worker метрики
curl http://localhost:9091/metrics
```

---

## Критерии производительности

### API Endpoints

| Endpoint | Target P95 | Max RPS |
|----------|------------|---------|
| /health | < 10ms | 10000+ |
| /api/v1/tournaments | < 100ms | 1000+ |
| /api/v1/tournaments/{id}/leaderboard | < 200ms | 500+ |
| /api/v1/auth/login | < 500ms | 100+ |

### Worker

| Метрика | Target |
|---------|--------|
| Match processing throughput | 100+ matches/sec |
| Queue dequeue latency | < 10ms |
| Autoscaling response time | < 5s |

### Database

| Операция | Target P95 |
|----------|------------|
| Simple SELECT | < 5ms |
| JOIN queries | < 50ms |
| Leaderboard refresh | < 500ms |

---

## Настройка окружения для тестов

### Минимальные требования

- CPU: 4 cores
- RAM: 8 GB
- Docker: 20.10+
- Go: 1.24+

### Рекомендуемые настройки PostgreSQL

```sql
-- Для нагрузочных тестов
ALTER SYSTEM SET max_connections = 200;
ALTER SYSTEM SET shared_buffers = '1GB';
ALTER SYSTEM SET work_mem = '64MB';
ALTER SYSTEM SET maintenance_work_mem = '512MB';
```

### Рекомендуемые настройки Redis

```
maxclients 10000
maxmemory 1gb
maxmemory-policy allkeys-lru
```

---

## Интерпретатор бенчмарков

Интерпретатор анализирует результаты и сравнивает с ожидаемыми значениями.

### Запуск

```bash
# Запуск бенчмарков с интерпретацией
make benchmark-interpret

# Показать ожидаемые стандарты
go run ./cmd/benchmark -standards

# Только интерпретация (без запуска тестов)
go run ./cmd/benchmark -interpret results.txt
```

### Пример вывода

```
=== TJudge Benchmark Results ===

API Benchmarks:
  Health Endpoint:    23µs (target: <50µs) ✓
  Tournament List:    4.2ms (target: <5ms) ✓
  Leaderboard:        8.5ms (target: <10ms) ✓

Worker Benchmarks:
  Throughput (100):   95ms (target: <100ms) ✓
  Autoscaling:        4.2s (target: <5s) ✓

Queue Benchmarks:
  Enqueue:            0.4ms (target: <0.5ms) ✓
  Dequeue:            0.3ms (target: <0.5ms) ✓

Overall: 7/7 passed ✓
```

---

## Устранение неполадок

### Тесты зависают

```bash
# Проверить доступность сервисов
curl http://localhost:8080/health
docker exec tjudge-redis redis-cli PING
docker exec tjudge-postgres pg_isready
```

### Низкий throughput

1. Проверить наличие rate limiting
2. Увеличить connection pool
3. Проверить нагрузку на БД
4. Проверить количество воркеров

### Высокий latency

1. Проверить индексы в БД
2. Проверить hit rate кэша
3. Профилировать запросы с pprof

---

## Профилирование

### CPU профиль

```bash
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

### Memory профиль

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Goroutine профиль

```bash
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Trace

```bash
curl -o trace.out http://localhost:6060/debug/pprof/trace?seconds=5
go tool trace trace.out
```

---

## Автоматизация в CI

Бенчмарки запускаются автоматически в CI pipeline:

```yaml
# .github/workflows/ci.yml
benchmark:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Run benchmarks
      run: make benchmark
    - name: Interpret results
      run: make benchmark-interpret
```

---

*Версия документации: 2.0*
*Последнее обновление: Январь 2026*
