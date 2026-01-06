# Loki Queries для TJudge

Коллекция полезных LogQL запросов для анализа логов TJudge системы.

## Базовые запросы

### Все логи API сервиса
```logql
{service="api"}
```

### Все логи Worker сервиса
```logql
{service="worker"}
```

### Логи с уровнем ERROR
```logql
{service=~"api|worker"} |= "error" | json | level="error"
```

### Логи с уровнем WARN или ERROR
```logql
{service=~"api|worker"} | json | level=~"warn|error"
```

## Фильтрация по контексту

### Логи конкретного турнира
```logql
{service=~"api|worker"} | json | tournament_id="<tournament-id>"
```

### Логи конкретного матча
```logql
{service=~"api|worker"} | json | match_id="<match-id>"
```

### Логи по request_id (трейсинг запроса)
```logql
{service="api"} | json | request_id="<request-id>"
```

### Логи конкретного пользователя
```logql
{service="api"} | json | user_id="<user-id>"
```

## Поиск по сообщениям

### Все логи с "failed"
```logql
{service=~"api|worker"} |= "failed"
```

### Логи о создании турниров
```logql
{service="api"} |= "tournament" |= "created"
```

### Логи о выполнении матчей
```logql
{service="worker"} |= "match" |= "executed"
```

### Database related errors
```logql
{service=~"api|worker"} |= "database" |= "error"
```

### Redis/Cache errors
```logql
{service=~"api|worker"} |= "redis" |= "error"
```

## Метрики и агрегация

### Количество ошибок в минуту
```logql
sum(rate({service=~"api|worker"} | json | level="error" [1m]))
```

### Количество логов по уровню (за последний час)
```logql
sum by (level) (count_over_time({service=~"api|worker"} | json [1h]))
```

### Количество матчей processed в секунду
```logql
rate({service="worker"} |= "match processed" [1m])
```

### Количество HTTP requests в секунду
```logql
rate({service="api"} |= "http request" [1m])
```

### Top 10 самых частых ошибок
```logql
topk(10,
  sum by (error) (
    count_over_time({service=~"api|worker"} | json | level="error" [1h])
  )
)
```

## Паттерн-матчинг

### Логи с timeout ошибками
```logql
{service=~"api|worker"} |~ "(?i)timeout|timed out"
```

### Логи с connection errors
```logql
{service=~"api|worker"} |~ "(?i)connection.*failed|failed.*connect"
```

### Логи с panic (критические ошибки)
```logql
{service=~"api|worker"} |~ "(?i)panic|fatal"
```

## Производительность

### Медленные запросы к БД (>1s)
```logql
{service=~"api|worker"}
  | json
  | duration > 1s
  | line_format "{{.msg}} duration={{.duration}}"
```

### Top 10 самых медленных операций
```logql
topk(10,
  avg by (operation) (
    {service=~"api|worker"}
      | json
      | unwrap duration
  )
)
```

### P95 latency для HTTP requests
```logql
quantile_over_time(0.95,
  {service="api"}
    | json
    | unwrap duration [5m]
) by (endpoint)
```

## Диагностика проблем

### Worker pool saturation (все воркеры заняты)
```logql
{service="worker"} |= "worker pool" |= "saturated"
```

### Queue overflow warnings
```logql
{service="worker"} |= "queue" |= "full"
```

### Out of memory errors
```logql
{service=~"api|worker"} |~ "(?i)out of memory|oom"
```

### Distributed lock conflicts
```logql
{service=~"api|worker"} |= "distributed lock" |= "conflict"
```

## Безопасность

### Failed authentication attempts
```logql
{service="api"} |= "auth" |= "failed"
```

### Unusual activity (400-500 status codes)
```logql
{service="api"}
  | json
  | status >= 400
```

### Rate limit hits
```logql
{service="api"} |= "rate limit" |= "exceeded"
```

## Комплексные запросы

### Errors per service with count
```logql
sum by (service, level) (
  count_over_time(
    {service=~"api|worker"}
      | json
      | level=~"error|warn" [1h]
  )
)
```

### Match processing pipeline
```logql
{service="worker"}
  | json
  | match_id!=""
  | line_format "{{.ts}} [{{.level}}] match={{.match_id}} {{.msg}}"
```

### Tournament lifecycle (создание -> старт -> завершение)
```logql
{service="api"}
  | json
  | tournament_id!=""
  | line_format "{{.ts}} tournament={{.tournament_id}} {{.msg}}"
```

## Экспорт и анализ

### Экспорт ошибок за последний час (для анализа)
```logql
{service=~"api|worker"}
  | json
  | level="error"
  | line_format "{{.ts}},{{.service}},{{.level}},{{.msg}},{{.error}}"
```

## Алертинг (для Prometheus Alertmanager)

### High error rate (>10 errors/min)
```logql
sum(rate({service=~"api|worker"} | json | level="error" [1m])) > 10
```

### Worker queue too large (>1000)
```logql
max_over_time(
  {service="worker"}
    | json
    | unwrap queue_size [5m]
) > 1000
```

### API high latency (p99 >2s)
```logql
quantile_over_time(0.99,
  {service="api"}
    | json
    | unwrap duration [5m]
) > 2
```

## Советы по использованию

1. **Индексы**: Используйте labels (service, level, stream) для быстрой фильтрации
2. **Временной диапазон**: Ограничивайте диапазон для лучшей производительности
3. **JSON parsing**: Применяйте `| json` только после фильтрации по тексту
4. **Регулярные выражения**: Используйте `|~` осторожно - это медленнее чем `|=`
5. **Агрегация**: Используйте `by()` для группировки по нужным полям

## Полезные поля в логах

- `ts` - timestamp
- `level` - уровень логирования (debug, info, warn, error)
- `msg` - сообщение
- `caller` - откуда вызван лог
- `error` - текст ошибки
- `service` - сервис (api, worker)
- `request_id` - ID запроса (для трейсинга)
- `user_id` - ID пользователя
- `tournament_id` - ID турнира
- `match_id` - ID матча
- `program_id` - ID программы
- `duration` - длительность операции
- `status` - HTTP status code
- `endpoint` - HTTP endpoint
- `operation` - тип операции (DB, Cache, etc.)
