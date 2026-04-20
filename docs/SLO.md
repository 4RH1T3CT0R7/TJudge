# TJudge - Service Level Objectives

Этот документ фиксирует пороги качества для мониторинга и алертинга. Значения стартовые, уточняются после 1-2 месяцев работы в проде.

## 1. SLI (Service Level Indicators)

| SLI | Источник | Описание |
|---|---|---|
| API availability | Prometheus: `up{job="api"}` | 1 = доступен, 0 = нет |
| API request success | `tjudge_http_requests_total{status!~"5.."}` / total | доля не-5xx ответов |
| API P99 latency | `tjudge_http_request_duration_seconds` histogram | 99-й перцентиль |
| Match processing latency | `tjudge_match_duration_seconds` | e2e от dequeue до результата |
| Queue oldest age | derived: max(`created_at` of pending match) | насколько старое застряло |
| WebSocket disconnect rate | derived: close-code 1008 per client | показатель флуда |
| Worker pool saturation | `tjudge_active_workers / tjudge_worker_pool_size` | перегрузка |

## 2. SLO (targets)

| Target | Порог | Окно измерения | Reaction |
|---|---|---|---|
| API availability | ≥ 99.5% | 28d rolling | Pagerduty CRIT |
| API 5xx rate | < 0.5% | 1h rolling | Slack WARN |
| API P99 latency | < 1s для read, < 3s для write | 1h rolling | Slack WARN |
| Health-endpoint P99 | < 100 ms | 5m | Pagerduty WARN |
| Match pickup lag | < 30s (85-й перц.) | 10m | Slack WARN |
| Queue oldest age | < 5 min | 5m | Slack CRIT |
| Leaderboard refresh P95 | < 5s | 15m | Slack WARN |
| Worker respawns | 0 / 24h | 24h | Pagerduty WARN (любая паника) |

## 3. Error budget

**Availability 99.5% за 28 дней** ≈ 3h 20m budget. Правила:
- Если budget выработан, запрет на feature-релизы; только bugfixes и reliability-фиксы.
- Sanctum: 25% budget (50m) - тревожный сигнал, ревью в следующем планировании.

## 4. Alerts (Prometheus и Alertmanager)

Примерные rules (`deployments/prometheus/alerts/*.yml`):

```yaml
- alert: APIHighErrorRate
  expr: sum(rate(tjudge_http_requests_total{status=~"5.."}[5m]))
        / sum(rate(tjudge_http_requests_total[5m])) > 0.005
  for: 10m
  labels: {severity: warning}
  annotations: {summary: "5xx rate > 0.5% for 10m"}

- alert: QueueStuck
  expr: tjudge_queue_size{priority="high"} > 500
  for: 5m
  labels: {severity: critical}

- alert: DeadLetterGrowing
  expr: increase(tjudge_queue_deadletter_push_total[10m]) > 10
  for: 10m
  labels: {severity: warning}

- alert: WorkerDrainTooSlow
  expr: histogram_quantile(0.99, rate(tjudge_worker_drain_duration_seconds_bucket[1h])) > 90
  labels: {severity: warning}

- alert: AuditLogLaggy
  expr: rate(audit_log_buffer_dropped_total[5m]) > 0
  labels: {severity: warning}
```

## 5. Performance benchmarks

Эталонные значения (`make benchmark-interpret`):

| Операция | Ожидается | Категория |
|---|---|---|
| Health endpoint | 50 µs | API |
| Список турниров | 5 ms | API |
| Leaderboard | 10 ms | API |
| Enqueue | 500 µs | Queue |
| Match create (БД) | 2 ms | DB |
| Worker pool 100 matches | 100 ms | Worker |

Регрессия > 10% в PR требует объяснения или апрува.

## 6. Security-SLO

| Target | Порог |
|---|---|
| 0 HIGH gosec findings | на каждом merge в main |
| 0 HIGH npm audit findings | на каждом merge |
| 0 HIGH Trivy findings | на каждом merge |
| Время до патча critical CVE | < 48h |

## 7. Capacity planning

- PostgreSQL: ~1 KB / match. При 1000 матчей в день получается ~35 MB в год. Retention не требуется на 5+ лет.
- Redis: queue и cache ~100 MB при нормальной нагрузке. Fatal при OOM; следить за `redis_memory_used_bytes`.
- Disk: `backups_data` растёт ~RETENTION_DAYS × ~среднего дампа. При > 10 GB нужно включить rclone sync.
