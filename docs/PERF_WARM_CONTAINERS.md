# P2.5 — Warm Docker-container pool для executor (design)

## Контекст

`internal/infrastructure/executor/executor.go` сейчас на каждый матч создаёт свежий Docker-контейнер. Cold start стоит 200–800 мс (зависит от образа и хоста). При 100+ матчах/сек это ~50–80% времени обработки.

## Цель

Держать пул "прогретых" контейнеров `tjudge-cli`, переиспользуя их между матчами. Ожидаемое улучшение — match-latency p95 с 900 мс → 200 мс.

## Ограничения безопасности

**НЕ используется pool-reuse, если:**
- Матч использовал сетевые флаги (`NetworkDisabled=false`) — не наш случай.
- Матч использовал bind-mount с user-программой в writable-режиме.
- В образе `tjudge-cli` есть глобальное состояние (файлы в `/tmp`, кэш, файл-дескрипторы).

**Обязательные инварианты:**
- После каждого матча состояние контейнера очищается: `docker exec rm -rf /workspace/*`.
- При непустом stderr матча — контейнер дропается и не возвращается в пул (defence-in-depth).
- seccomp, AppArmor, cpu/memory limits — выставляются при создании, не меняются.

## Архитектура

```
┌──────────────────────┐
│  MatchProcessor      │
│  ┌───────────────┐   │
│  │ Acquire(ctx)  │──▶│  ContainerPool
│  └───────────────┘   │  ┌───────────────┐
│  ┌───────────────┐   │  │ free: chan    │
│  │ RunMatch(...) │◀─▶│  │ busy: map     │
│  └───────────────┘   │  │ gauges: ...   │
│  ┌───────────────┐   │  └───────────────┘
│  │ Release/Drop  │──▶│
│  └───────────────┘   │
└──────────────────────┘
```

## Go-интерфейс

```go
// internal/infrastructure/executor/pool.go
type ContainerPool interface {
    // Acquire возвращает готовый контейнер (prewarmed).
    // Если free-очередь пуста, создаёт новый до max_size.
    Acquire(ctx context.Context) (*PooledContainer, error)

    // Release возвращает контейнер в пул для reuse.
    Release(c *PooledContainer)

    // Drop помечает контейнер "нечистым", останавливает и убирает из пула.
    Drop(c *PooledContainer)

    // Stats возвращает текущее состояние для метрик.
    Stats() PoolStats
}

type PooledContainer struct {
    ID      string
    Created time.Time
    UsedBy  uuid.UUID // match id, для логов
}

type PoolStats struct {
    Free, Busy, TotalCreated int
}
```

## Поэтапная миграция

1. **Этап 1 (safe):** интерфейс + два реализации:
   - `noReusePool{}` — текущее поведение, один контейнер на матч. Default.
   - `reusePool{}` — warm-reuse, активируется через `EXECUTOR_POOL_REUSE=true`.
2. **Этап 2:** добавить prewarm — при старте executor'а держать `EXECUTOR_POOL_MIN` уже запущенных контейнеров.
3. **Этап 3:** adaptive sizing — динамически расти до `EXECUTOR_POOL_MAX` по нагрузке.

## Метрики

- `tjudge_executor_pool_free` — gauge, размер free-очереди.
- `tjudge_executor_pool_busy` — gauge, сколько сейчас занято.
- `tjudge_executor_container_acquire_duration_seconds` — histogram.
- `tjudge_executor_container_dropped_total{reason}` — counter (`dirty_state`, `error`, `expired`).

## Риски

| Риск | Митигация |
|---|---|
| Утечка состояния между матчами | Workspace-cleanup + drop-on-error; audit команды cleanup в тестах. |
| Zombie-контейнеры при панике процесса | `docker ps -f label=tjudge-pool=1` на старте → kill. |
| Resource leak при bug'е в Release | Circuit-breaker: если free > max, лишние killятся. |

## Тест-план

- Unit: `TestPool_AcquireReleaseRoundTrip`, `TestPool_DropRemoves`, `TestPool_MaxSizeCap`.
- Integration: бенчмарк noReuse vs reuse — `go test -tags=integration -bench=. ./internal/infrastructure/executor/`.
- Chaos: `docker kill` случайного контейнера во время работы pool → re-use continues cleanly.

## Итог

Дизайн готов для реализации. Предлагаемая последовательность работ:
1. Реализовать `ContainerPool` interface + 2 реализации (~1 день).
2. Интегрировать в `executor.go` через feature-flag (~0.5 дня).
3. Бенчмарк и tuning (~1 день).
4. Выкатить с `EXECUTOR_POOL_REUSE=true` в stage, затем в prod.

Total: ~3 дня.
