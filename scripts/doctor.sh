#!/bin/bash
set -uo pipefail

# ============================================================================
# TJudge Doctor - умная пост-деплойная диагностика.
#
#   make doctor                       # человекочитаемый отчёт в терминал
#   ./scripts/doctor.sh --json        # машиночитаемый JSON
#   DOCTOR_TELEGRAM=always make doctor  # отчёт в Telegram даже когда всё ок
#
# Что проверяет (каждая проверка независима, с подсказкой «куда смотреть»):
#   1. Контейнеры: запущены, healthy, не рестартятся в цикле
#   2. Образы: tjudge-cli/tjudge-builder существуют; api/worker не работают
#      на устаревшем образе (пересобран, но не перезапущен)
#   3. Health-эндпоинты API и worker'а
#   4. Полный статус /system/status (ADMIN_TOKEN): БД+миграции, Redis,
#      dead-letter, outbox целостности рейтингов, зависшая компиляция
#   5. Prometheus: целевые up-метрики, 5xx-rate, активные алерты
#      (firing-алерты попадают в отчёт как есть - с их описаниями)
#   6. Логи api/worker за DOCTOR_LOG_WINDOW: количество error/panic,
#      топ повторяющихся сообщений
#   7. Диск
#
# Куда отчитывается:
#   - терминал (всегда), exit-код: 0 здорово/предупреждения, 1 критично
#   - Telegram (TELEGRAM_BOT_TOKEN/TELEGRAM_CHAT_ID): при проблемах,
#     или всегда при DOCTOR_TELEGRAM=always; never - отключить
#   - Pushgateway (PUSHGATEWAY_URL): метрики чеков для дашборда
#     «TJudge - Doctor» в Grafana и алертов Prometheus
#
# Конфиг (ENV или .env):
#   API_URL=http://localhost:8080
#   WORKER_METRICS_URL=http://localhost:9090
#   PROMETHEUS_URL=http://localhost:9092
#   PUSHGATEWAY_URL=http://localhost:9094      # дефолт; недоступен - тихий скип
#   ADMIN_USER=<логин или email админа>        # авто-логин для /system/status
#   ADMIN_PASSWORD=<пароль>
#   ADMIN_TOKEN=<jwt>                          # альтернатива: готовый токен
#                                              # (истекает за JWT_ACCESS_TTL)
#   DOCTOR_TELEGRAM=auto|always|never          # default auto
#   DOCTOR_LOG_WINDOW=15m
#   DOCTOR_LOG_ERROR_THRESHOLD=5               # ошибок в окне до warn
#   DOCTOR_MAX_RESTARTS=0                      # рестартов контейнера до warn
#   DOCTOR_DISK_WARN=85 DOCTOR_DISK_CRIT=95    # % использования диска
#   DOCTOR_FAIL_ON_WARN=false                  # exit 1 и на warn
# ============================================================================

cd "$(dirname "$0")/.." 2>/dev/null || true

if [ -f .env ]; then set -a; . ./.env 2>/dev/null; set +a; fi

API_URL="${API_URL:-http://localhost:8080}"
WORKER_METRICS_URL="${WORKER_METRICS_URL:-http://localhost:9090}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9092}"
# Дефолт - стандартный порт pushgateway из infra-monitoring (127.0.0.1:9094);
# если его нет (dev-машина), push тихо скипается с одной строчкой в выводе.
PUSHGATEWAY_URL="${PUSHGATEWAY_URL:-http://localhost:9094}"
DOCTOR_TELEGRAM="${DOCTOR_TELEGRAM:-auto}"
DOCTOR_LOG_WINDOW="${DOCTOR_LOG_WINDOW:-15m}"
DOCTOR_LOG_ERROR_THRESHOLD="${DOCTOR_LOG_ERROR_THRESHOLD:-5}"
DOCTOR_MAX_RESTARTS="${DOCTOR_MAX_RESTARTS:-0}"
DOCTOR_DISK_WARN="${DOCTOR_DISK_WARN:-85}"
DOCTOR_DISK_CRIT="${DOCTOR_DISK_CRIT:-95}"
DOCTOR_FAIL_ON_WARN="${DOCTOR_FAIL_ON_WARN:-false}"

JSON_MODE=false
[ "${1:-}" = "--json" ] && JSON_MODE=true

G=$'\033[0;32m'; R=$'\033[0;31m'; Y=$'\033[1;33m'; C=$'\033[0;36m'; B=$'\033[1m'; X=$'\033[0m'

# Результаты: параллельные массивы (bash 3 совместимость, без assoc arrays).
SEVERITIES=(); CHECKS=(); MESSAGES=(); HINTS=()

add() { # add <ok|warn|crit> <check> <message> [hint]
    SEVERITIES+=("$1"); CHECKS+=("$2"); MESSAGES+=("$3"); HINTS+=("${4:-}")
}

have() { command -v "$1" >/dev/null 2>&1; }

# ---------------------------------------------------------------- 1. контейнеры
check_containers() {
    have docker || { add warn containers "docker недоступен - проверка контейнеров пропущена"; return; }

    local names
    names=$(docker ps -a --format '{{.Names}}' 2>/dev/null | grep -E '^tjudge-' || true)
    if [ -z "$names" ]; then
        add warn containers "контейнеры tjudge-* не найдены" "если это dev-машина без compose - норм; на сервере: docker compose up -d"
        return
    fi

    local all_ok=true
    while IFS= read -r name; do
        local state health restarts
        state=$(docker inspect "$name" --format '{{.State.Status}}' 2>/dev/null)
        health=$(docker inspect "$name" --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' 2>/dev/null)
        restarts=$(docker inspect "$name" --format '{{.RestartCount}}' 2>/dev/null)

        if [ "$state" = "exited" ]; then
            # One-shot задачи (migrate и т.п.): exited с кодом 0 - нормальное
            # завершение, а не сбой. Сбой - только ненулевой exit-код.
            local exit_code policy
            exit_code=$(docker inspect "$name" --format '{{.State.ExitCode}}' 2>/dev/null)
            policy=$(docker inspect "$name" --format '{{.HostConfig.RestartPolicy.Name}}' 2>/dev/null)
            if [ "${exit_code:-1}" = "0" ] && { [ "$policy" = "no" ] || [ "$policy" = "on-failure" ] || [ -z "$policy" ]; }; then
                add ok "container:$name" "one-shot задача завершилась успешно (exit 0)"
            else
                add crit "container:$name" "завершился с кодом ${exit_code:-?}" "docker logs --tail=100 $name"
                all_ok=false
            fi
        elif [ "$state" != "running" ]; then
            add crit "container:$name" "статус $state" "docker logs --tail=100 $name; docker compose up -d $name"
            all_ok=false
        elif [ "$health" = "starting" ]; then
            # Healthcheck ещё в start_period (сразу после деплоя) - не сбой.
            add warn "container:$name" "health=starting (ещё проходит первый healthcheck)" "повторите make doctor через минуту"
            all_ok=false
        elif [ -n "$health" ] && [ "$health" != "healthy" ]; then
            add crit "container:$name" "запущен, но health=$health" "docker inspect $name --format '{{json .State.Health}}' | jq"
            all_ok=false
        elif [ "${restarts:-0}" -gt "$DOCTOR_MAX_RESTARTS" ]; then
            add warn "container:$name" "перезапусков: $restarts (возможен crash-loop)" "docker logs --tail=200 $name | grep -iE 'panic|fatal'"
            all_ok=false
        fi
    done <<< "$names"

    $all_ok && add ok containers "все tjudge-контейнеры запущены и healthy"
}

# -------------------------------------------------------------------- 2. образы
check_images() {
    have docker || return 0

    for entry in "tjudge-cli:latest=make docker-build-executor" "tjudge-builder:latest=make docker-build-builder"; do
        local img="${entry%%=*}" hint="${entry#*=}"
        if docker image inspect "$img" >/dev/null 2>&1; then
            add ok "image:${img%%:*}" "образ есть"
        else
            add crit "image:${img%%:*}" "образ $img отсутствует" "$hint (без него матчи/компиляция не работают)"
        fi
    done

    # Контейнер на устаревшем образе: образ пересобрали, контейнер не перезапустили.
    for ctr in $(docker ps --format '{{.Names}}' 2>/dev/null | grep -E '^tjudge-?.*(api|worker)' || true); do
        local running tag latest
        running=$(docker inspect "$ctr" --format '{{.Image}}' 2>/dev/null)
        tag=$(docker inspect "$ctr" --format '{{.Config.Image}}' 2>/dev/null)
        latest=$(docker image inspect "$tag" --format '{{.Id}}' 2>/dev/null)
        if [ -n "$running" ] && [ -n "$latest" ] && [ "$running" != "$latest" ]; then
            add warn "stale-image:$ctr" "работает на устаревшем образе $tag" "образ пересобран, контейнер нет: docker compose up -d $ctr"
        fi
    done
}

# -------------------------------------------------------------------- 3. health
check_health() {
    if curl -sf --max-time 5 "$API_URL/health" >/dev/null 2>&1; then
        add ok api-health "API отвечает ($API_URL/health)"
    else
        add crit api-health "API недоступен ($API_URL/health)" "docker logs --tail=100 tjudge-api"
    fi

    if curl -sf --max-time 5 "$WORKER_METRICS_URL/health" >/dev/null 2>&1; then
        add ok worker-health "worker отвечает ($WORKER_METRICS_URL/health)"
    elif have docker && docker ps --format '{{.Names}}' 2>/dev/null | grep -qE '^tjudge-worker|^tjudge.*worker'; then
        # В prod метрики-порт worker'а не публикуется на хост (expose-only) -
        # это норма; здоровье воркера подтверждаем по состоянию контейнера,
        # а скрейп метрик проверяет Prometheus-чек ниже.
        add ok worker-health "worker запущен (метрики-порт не опубликован на хост - норма для prod)"
    else
        add warn worker-health "worker health недоступен ($WORKER_METRICS_URL/health) и контейнер не найден" "docker compose up -d worker; METRICS_ENABLED/METRICS_PORT в .env"
    fi
}

# ------------------------------------------------------------- 4. /system/status
check_system_status() {
    have jq || { add warn system-status "jq не установлен - разбор /system/status пропущен"; return; }

    # Авто-логин: свежий JWT на каждый запуск, готовый ADMIN_TOKEN в .env
    # протухает за JWT_ACCESS_TTL.
    if [ -z "${ADMIN_TOKEN:-}" ] && [ -n "${ADMIN_USER:-}" ] && [ -n "${ADMIN_PASSWORD:-}" ]; then
        local login_field="username"
        case "$ADMIN_USER" in *@*) login_field="email" ;; esac
        ADMIN_TOKEN=$(jq -n --arg f "$login_field" --arg u "$ADMIN_USER" --arg p "$ADMIN_PASSWORD" '{($f): $u, password: $p}' \
            | curl -sf --max-time 5 -H 'Content-Type: application/json' -d @- "$API_URL/api/v1/auth/login" 2>/dev/null \
            | jq -r '.data.access_token // empty')
        if [ -z "$ADMIN_TOKEN" ]; then
            add warn system-status "авто-логин под $ADMIN_USER не удался" "проверьте ADMIN_USER/ADMIN_PASSWORD в .env и роль admin у учётки"
            return
        fi
    fi

    if [ -z "${ADMIN_TOKEN:-}" ]; then
        add warn system-status "нет доступа - глубокая проверка БД/очередей/outbox пропущена" "задайте ADMIN_USER и ADMIN_PASSWORD (учётка админа) в .env"
        return
    fi

    local body
    body=$(curl -sf --max-time 8 -H "Authorization: Bearer $ADMIN_TOKEN" "$API_URL/api/v1/system/status" 2>/dev/null)
    if [ -z "$body" ]; then
        add crit system-status "не удалось получить /system/status" "токен истёк или нет admin-роли; curl -v с тем же токеном"
        return
    fi

    q() { echo "$body" | jq -r "$1 // empty"; }

    [ "$(q '.data.database.healthy')" = "true" ] \
        && add ok db "PostgreSQL healthy, миграция $(q '.data.database.schema_version')" \
        || add crit db "PostgreSQL unhealthy" "docker logs --tail=100 tjudge-postgres; проверьте DB_* в .env"

    [ "$(q '.data.database.schema_dirty')" = "true" ] \
        && add crit db-schema "миграции в состоянии DIRTY" "незавершённая миграция: golang-migrate требует ручного вмешательства (docs/DATABASE_SCHEMA.md)"

    [ "$(q '.data.redis.healthy')" = "true" ] \
        && add ok redis "Redis healthy" \
        || add crit redis "Redis unhealthy" "docker logs --tail=100 tjudge-redis"

    local dl oe op cq
    dl=$(q '.data.queues.dead_letter'); oe=$(q '.data.outbox.errors')
    op=$(q '.data.outbox.pending');    cq=$(q '.data.queues.compile')

    [ -n "$dl" ] && [ "$dl" -gt 0 ] 2>/dev/null \
        && add warn dead-letter "dead-letter очередь: $dl задач" "повреждённые задачи; docker logs tjudge-worker | grep dead-letter"
    [ -n "$oe" ] && [ "$oe" -gt 0 ] 2>/dev/null \
        && add crit outbox "outbox errors: $oe - рейтинги могли не примениться" "SELECT * FROM match_outbox WHERE status='error' - смотрите last_error"
    [ -n "$op" ] && [ "$op" -gt 10 ] 2>/dev/null \
        && add warn outbox-backlog "outbox pending: $op (диспетчер не успевает?)" "проверьте логи worker'а: OutboxDispatcher"
    [ -n "$cq" ] && [ "$cq" -gt 5 ] 2>/dev/null \
        && add warn compile-queue "очередь компиляции: $cq" "builder-образ есть? docker logs tjudge-worker | grep -i compile"

    local compiling
    compiling=$(echo "$body" | jq -r '.data.programs.compiling // 0')
    [ "$compiling" -gt 10 ] 2>/dev/null \
        && add warn programs-compiling "$compiling программ зависли в compiling" "stuck-recovery вернёт их в очередь; если не уменьшается - смотрите compile-worker"

    add ok system-status "полный статус получен (версия $(q '.data.app.version'), аптайм $(( $(q '.data.app.uptime_seconds') / 3600 ))ч)"
}

# ----------------------------------------------------------------- 5. prometheus
prom_query() { # prom_query <promql> -> значение первого результата или ""
    curl -sf --max-time 5 "$PROMETHEUS_URL/api/v1/query" --data-urlencode "query=$1" 2>/dev/null \
        | jq -r '.data.result[0].value[1] // empty' 2>/dev/null
}

check_prometheus() {
    have jq || return 0
    if ! curl -sf --max-time 5 "$PROMETHEUS_URL/-/ready" >/dev/null 2>&1; then
        add warn prometheus "Prometheus недоступен ($PROMETHEUS_URL) - метрики-проверки пропущены" "профиль monitoring поднят? docker compose --profile monitoring up -d"
        return
    fi

    local v
    v=$(prom_query 'max(up{job=~"tjudge-api|api"})')
    [ "$v" = "1" ] && add ok prom-api-up "Prometheus видит API" \
        || add warn prom-api-up "Prometheus НЕ скрейпит API (up=${v:-нет данных})" "мониторинг-стек поднят и в одной сети с api? docker compose -f docker-compose.monitoring.yml up -d (см. docs/OPERATIONS.md §5)"

    v=$(prom_query 'max(up{job=~"tjudge-worker|worker"})')
    [ "$v" = "1" ] && add ok prom-worker-up "Prometheus видит worker" \
        || add warn prom-worker-up "Prometheus НЕ скрейпит worker (up=${v:-нет данных})" "METRICS_PORT=9090 у worker'а?"

    # 5xx-rate за 10м против SLO 0.5%.
    v=$(prom_query 'sum(rate(tjudge_http_requests_total{status=~"5.."}[10m])) / clamp_min(sum(rate(tjudge_http_requests_total[10m])), 0.0001)')
    if [ -n "$v" ]; then
        if awk "BEGIN{exit !($v > 0.005)}"; then
            add crit error-rate "5xx-rate $(awk "BEGIN{printf \"%.2f%%\", $v*100}") > SLO 0.5%" "Grafana: панель «5xx rate»; docker logs tjudge-api | grep '\"status\":5'"
        else
            add ok error-rate "5xx-rate в норме ($(awk "BEGIN{printf \"%.3f%%\", $v*100}"))"
        fi
    fi

    # Активные алерты Prometheus - готовые «на что обратить внимание».
    local alerts
    alerts=$(curl -sf --max-time 5 "$PROMETHEUS_URL/api/v1/alerts" 2>/dev/null \
        | jq -r '.data.alerts[]? | select(.state=="firing") | "\(.labels.alertname)|\(.labels.severity // "warning")|\(.annotations.summary // "")"' 2>/dev/null)
    if [ -n "$alerts" ]; then
        while IFS='|' read -r aname asev asum; do
            [ -z "$aname" ] && continue
            local sev=warn; [ "$asev" = "critical" ] && sev=crit
            add "$sev" "alert:$aname" "${asum:-алерт активен}" "Prometheus → Alerts; описание порога в deployments/prometheus/alerts/"
        done <<< "$alerts"
    else
        add ok prom-alerts "активных алертов нет"
    fi
}

# ----------------------------------------------------------------------- 6. логи
check_logs() {
    have docker || return 0

    local log_ctrs
    log_ctrs=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E '^tjudge-?.*(api|worker)' || true)
    for ctr in $log_ctrs; do

        local logs errors panics
        logs=$(docker logs --since "$DOCTOR_LOG_WINDOW" "$ctr" 2>&1 || true)
        errors=$(echo "$logs" | grep -c '"level":"error"' || true)
        panics=$(echo "$logs" | grep -ciE 'panic:|fatal' || true)

        if [ "${panics:-0}" -gt 0 ]; then
            local sample
            sample=$(echo "$logs" | grep -iE 'panic:|fatal' | head -1 | cut -c1-160)
            add crit "logs:$ctr" "panic/fatal за $DOCTOR_LOG_WINDOW: $panics («$sample»)" "docker logs --tail=300 $ctr"
        elif [ "${errors:-0}" -gt "$DOCTOR_LOG_ERROR_THRESHOLD" ]; then
            local top
            top=$(echo "$logs" | grep '"level":"error"' \
                | (have jq && jq -r '.msg // empty' 2>/dev/null || sed 's/.*"msg":"\([^"]*\)".*/\1/') \
                | sort | uniq -c | sort -rn | head -3 | sed 's/^ *//' | tr '\n' '; ')
            add warn "logs:$ctr" "ошибок в логах за $DOCTOR_LOG_WINDOW: $errors (топ: $top)" "docker logs --since $DOCTOR_LOG_WINDOW $ctr | grep '\"level\":\"error\"'"
        else
            add ok "logs:$ctr" "логи чистые за $DOCTOR_LOG_WINDOW (ошибок: ${errors:-0})"
        fi
    done
}

# ----------------------------------------------------------------------- 7. диск
check_disk() {
    local usage
    usage=$(df -P . 2>/dev/null | awk 'NR==2 {gsub("%",""); print $5}')
    [ -z "$usage" ] && return 0

    if [ "$usage" -ge "$DOCTOR_DISK_CRIT" ]; then
        add crit disk "диск заполнен на ${usage}%" "docker system prune (ОСТОРОЖНО: см. docs/OPERATIONS.md §7.1 - не -af!); старые бэкапы; партиции БД"
    elif [ "$usage" -ge "$DOCTOR_DISK_WARN" ]; then
        add warn disk "диск заполнен на ${usage}%" "посмотрите du -sh backups/ data/ и docker images"
    else
        add ok disk "диск: ${usage}%"
    fi
}

# ============================================================== запуск проверок
check_containers
check_images
check_health
check_system_status
check_prometheus
check_logs
check_disk

# ================================================================== вердикт
CRITS=0; WARNS=0; OKS=0
for s in "${SEVERITIES[@]}"; do
    case "$s" in crit) CRITS=$((CRITS+1));; warn) WARNS=$((WARNS+1));; ok) OKS=$((OKS+1));; esac
done

VERDICT="HEALTHY"; [ "$WARNS" -gt 0 ] && VERDICT="DEGRADED"; [ "$CRITS" -gt 0 ] && VERDICT="CRITICAL"

# ------------------------------------------------------------------ вывод JSON
if $JSON_MODE; then
    {
        printf '{"verdict":"%s","ok":%d,"warn":%d,"crit":%d,"checks":[' "$VERDICT" "$OKS" "$WARNS" "$CRITS"
        for i in "${!SEVERITIES[@]}"; do
            [ "$i" -gt 0 ] && printf ','
            printf '{"severity":"%s","check":"%s","message":%s,"hint":%s}' \
                "${SEVERITIES[$i]}" "${CHECKS[$i]}" \
                "$(printf '%s' "${MESSAGES[$i]}" | jq -Rs .)" \
                "$(printf '%s' "${HINTS[$i]}" | jq -Rs .)"
        done
        printf ']}\n'
    }
else
    printf "\n%b┌─ TJudge Doctor ──────────────────────────────────────────┐%b\n" "$C$B" "$X"
    for i in "${!SEVERITIES[@]}"; do
        case "${SEVERITIES[$i]}" in
            ok)   printf "  %b✓%b %-22s %s\n" "$G" "$X" "${CHECKS[$i]}" "${MESSAGES[$i]}" ;;
            warn) printf "  %b!%b %-22s %s\n" "$Y" "$X" "${CHECKS[$i]}" "${MESSAGES[$i]}"
                  [ -n "${HINTS[$i]}" ] && printf "      %b→ %s%b\n" "$Y" "${HINTS[$i]}" "$X" ;;
            crit) printf "  %b✗%b %-22s %s\n" "$R" "$X" "${CHECKS[$i]}" "${MESSAGES[$i]}"
                  [ -n "${HINTS[$i]}" ] && printf "      %b→ %s%b\n" "$R" "${HINTS[$i]}" "$X" ;;
        esac
    done
    printf "%b└──────────────────────────────────────────────────────────┘%b\n" "$C$B" "$X"
    case "$VERDICT" in
        HEALTHY)  printf "  %bВердикт: HEALTHY%b (ok: %d)\n\n" "$G$B" "$X" "$OKS" ;;
        DEGRADED) printf "  %bВердикт: DEGRADED%b (warn: %d, ok: %d)\n\n" "$Y$B" "$X" "$WARNS" "$OKS" ;;
        CRITICAL) printf "  %bВердикт: CRITICAL%b (crit: %d, warn: %d, ok: %d)\n\n" "$R$B" "$X" "$CRITS" "$WARNS" "$OKS" ;;
    esac
fi

# -------------------------------------------------------------------- telegram
send_telegram() {
    [ "$DOCTOR_TELEGRAM" = "never" ] && return 0
    [ -z "${TELEGRAM_BOT_TOKEN:-}" ] || [ -z "${TELEGRAM_CHAT_ID:-}" ] && return 0
    if [ "$DOCTOR_TELEGRAM" = "auto" ] && [ "$VERDICT" = "HEALTHY" ]; then return 0; fi

    local icon="✅"; [ "$VERDICT" = "DEGRADED" ] && icon="⚠️"; [ "$VERDICT" = "CRITICAL" ] && icon="🔥"
    local msg="$icon <b>TJudge Doctor: $VERDICT</b> ($(hostname), $(date '+%Y-%m-%d %H:%M'))"
    msg="$msg
ok: $OKS, warn: $WARNS, crit: $CRITS"

    for i in "${!SEVERITIES[@]}"; do
        local s="${SEVERITIES[$i]}"
        [ "$s" = "ok" ] && continue
        local mark="⚠️"; [ "$s" = "crit" ] && mark="❌"
        msg="$msg
$mark <b>${CHECKS[$i]}</b>: ${MESSAGES[$i]}"
        [ -n "${HINTS[$i]}" ] && msg="$msg
   → ${HINTS[$i]}"
    done

    # Telegram режет сообщения > 4096 символов.
    msg=$(printf '%s' "$msg" | head -c 3900)

    curl -sS --max-time 15 "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
        -d "chat_id=${TELEGRAM_CHAT_ID}" \
        --data-urlencode "text=${msg}" \
        -d "parse_mode=HTML" >/dev/null 2>&1 \
        && { $JSON_MODE || echo "  отчёт отправлен в Telegram"; } \
        || { $JSON_MODE || echo "  не удалось отправить в Telegram"; }
}
send_telegram

# ----------------------------------------------------------------- pushgateway
push_metrics() {
    [ -z "$PUSHGATEWAY_URL" ] && return 0

    local payload="# TYPE tjudge_doctor_check gauge
"
    for i in "${!SEVERITIES[@]}"; do
        # Лейблы: только [a-zA-Z0-9_:-] в имени чека.
        local name
        name=$(printf '%s' "${CHECKS[$i]}" | tr -c 'a-zA-Z0-9_:-' '_')
        local val=0; [ "${SEVERITIES[$i]}" = "ok" ] && val=1
        payload="${payload}tjudge_doctor_check{check=\"${name}\",severity=\"${SEVERITIES[$i]}\"} ${val}
"
    done
    payload="${payload}# TYPE tjudge_doctor_problems gauge
tjudge_doctor_problems{severity=\"warn\"} ${WARNS}
tjudge_doctor_problems{severity=\"crit\"} ${CRITS}
# TYPE tjudge_doctor_verdict gauge
tjudge_doctor_verdict $([ "$VERDICT" = "HEALTHY" ] && echo 0 || { [ "$VERDICT" = "DEGRADED" ] && echo 1 || echo 2; })
# TYPE tjudge_doctor_last_run_timestamp_seconds gauge
tjudge_doctor_last_run_timestamp_seconds $(date +%s)
"

    printf '%s' "$payload" | curl -sS --max-time 10 --data-binary @- \
        "$PUSHGATEWAY_URL/metrics/job/tjudge_doctor" >/dev/null 2>&1 \
        && { $JSON_MODE || echo "  метрики отправлены в Pushgateway"; } \
        || { $JSON_MODE || echo "  Pushgateway недоступен ($PUSHGATEWAY_URL)"; }
}
push_metrics

# ---------------------------------------------------------------------- exit
[ "$CRITS" -gt 0 ] && exit 1
[ "$DOCTOR_FAIL_ON_WARN" = "true" ] && [ "$WARNS" -gt 0 ] && exit 1
exit 0
