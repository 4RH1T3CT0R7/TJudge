#!/bin/bash
set -uo pipefail

# TJudge: полное состояние системы в терминале.
#
#   make status            - локально/на сервере рядом с docker-compose
#   ./scripts/status.sh
#
# Конфиг через ENV (или .env в корне):
#   API_URL        - адрес API (по умолчанию http://localhost:8080)
#   ADMIN_TOKEN    - JWT админа: добавляет секцию полного статуса
#                    (очереди, матчи, программы, outbox) из /system/status
#
# Что проверяется:
#   1. Контейнеры (docker compose ps)
#   2. Образы: наличие tjudge-cli/tjudge-builder и НЕ устарели ли запущенные
#      контейнеры относительно локально собранных образов (нужен ли restart)
#   3. API /health и worker /health
#   4. Полный статус из API (с ADMIN_TOKEN): БД, миграции, Redis, очереди,
#      матчи, программы, outbox, WebSocket

API_URL="${API_URL:-http://localhost:8080}"
WORKER_METRICS_URL="${WORKER_METRICS_URL:-http://localhost:9090}"

# Подхватываем .env, если есть (для ADMIN_TOKEN/портов).
if [ -f .env ]; then
    set -a; . ./.env 2>/dev/null; set +a
fi

B=$(tput bold 2>/dev/null || true)
N=$(tput sgr0 2>/dev/null || true)
G=$'\033[0;32m'; R=$'\033[0;31m'; Y=$'\033[1;33m'; C=$'\033[0;36m'; X=$'\033[0m'

ok()   { printf "  %b●%b %s\n" "$G" "$X" "$1"; }
bad()  { printf "  %b●%b %s\n" "$R" "$X" "$1"; }
warn() { printf "  %b●%b %s\n" "$Y" "$X" "$1"; }
hdr()  { printf "\n%b%s%b\n" "$C$B" "$1" "$X"; }

FAILURES=0

# ---------------------------------------------------------------- контейнеры
hdr "── Контейнеры ────────────────────────────────────────────"
if command -v docker >/dev/null 2>&1; then
    if docker compose ps --format '{{.Name}}\t{{.Status}}' 2>/dev/null | grep -q .; then
        docker compose ps --format '{{.Name}}\t{{.Status}}' 2>/dev/null | while IFS=$'\t' read -r name st; do
            case "$st" in
                Up*\(healthy\)*|Up*) ok "$name — $st" ;;
                *) printf "  %b●%b %s — %s\n" "$R" "$X" "$name" "$st" ;;
            esac
        done
    else
        warn "docker compose не видит сервисов в этом каталоге (это нормально для dev без compose)"
    fi
else
    warn "docker не установлен/недоступен"
fi

# ------------------------------------------------------------------- образы
hdr "── Образы ────────────────────────────────────────────────"
if command -v docker >/dev/null 2>&1; then
    for entry in "tjudge-cli:latest=make docker-build-executor" "tjudge-builder:latest=make docker-build-builder"; do
        img="${entry%%=*}"; hint="${entry#*=}"
        if created=$(docker image inspect "$img" --format '{{.Created}}' 2>/dev/null); then
            ok "$img (создан: ${created%T*} ${created:11:8})"
        else
            bad "$img ОТСУТСТВУЕТ — соберите: $hint"
            FAILURES=$((FAILURES+1))
        fi
    done

    # Контейнер запущен на устаревшем образе? Сравниваем image ID запущенного
    # контейнера с ID локального образа того же тега: разошлись — образ
    # пересобран, но контейнер не перезапущен (нужен docker compose up -d).
    for ctr in $(docker ps --format '{{.Names}}' 2>/dev/null | grep -E '^tjudge-(api|worker)$' || true); do
        running_img=$(docker inspect "$ctr" --format '{{.Image}}' 2>/dev/null)
        tag=$(docker inspect "$ctr" --format '{{.Config.Image}}' 2>/dev/null)
        latest_img=$(docker image inspect "$tag" --format '{{.Id}}' 2>/dev/null)
        if [ -n "$running_img" ] && [ -n "$latest_img" ]; then
            if [ "$running_img" = "$latest_img" ]; then
                ok "$ctr работает на актуальном образе ($tag)"
            else
                warn "$ctr работает на УСТАРЕВШЕМ образе — образ $tag пересобран, контейнер не перезапущен (docker compose up -d $ctr)"
                FAILURES=$((FAILURES+1))
            fi
        fi
    done
fi

# ------------------------------------------------------------------ healthz
hdr "── Health ────────────────────────────────────────────────"
if resp=$(curl -sf --max-time 3 "$API_URL/health" 2>/dev/null); then
    ok "API $API_URL/health"
else
    bad "API $API_URL/health недоступен"
    FAILURES=$((FAILURES+1))
fi

if curl -sf --max-time 3 "$WORKER_METRICS_URL/health" >/dev/null 2>&1; then
    ok "Worker $WORKER_METRICS_URL/health"
else
    warn "Worker $WORKER_METRICS_URL/health недоступен (METRICS_ENABLED=false или другой порт)"
fi

# --------------------------------------------------------- полный статус API
hdr "── Полный статус (/system/status) ────────────────────────"
if [ -z "${ADMIN_TOKEN:-}" ]; then
    warn "ADMIN_TOKEN не задан — пропуск. Получить: ADMIN_TOKEN=\$(curl -s $API_URL/api/v1/auth/login -d '{\"username\":\"...\",\"password\":\"...\"}' -H 'Content-Type: application/json' | jq -r .data.access_token)"
else
    status=$(curl -sf --max-time 5 -H "Authorization: Bearer $ADMIN_TOKEN" "$API_URL/api/v1/system/status" 2>/dev/null)
    if [ -z "$status" ]; then
        bad "не удалось получить /system/status (токен истёк? нет admin-роли?)"
        FAILURES=$((FAILURES+1))
    elif ! command -v jq >/dev/null 2>&1; then
        warn "jq не установлен — сырой JSON:"
        echo "$status"
    else
        d() { echo "$status" | jq -r "$1 // empty"; }

        ver=$(d '.data.app.version'); dirty=$(d '.data.app.dirty')
        up=$(d '.data.app.uptime_seconds')
        printf "  %bВерсия:%b %s%s   %bGo:%b %s   %bАптайм:%b %dд %dч %dм\n" \
            "$B" "$N" "$ver" "$([ "$dirty" = "true" ] && printf '%b-dirty%b' "$Y" "$X")" \
            "$B" "$N" "$(d '.data.app.go_version')" \
            "$B" "$N" $((up/86400)) $((up%86400/3600)) $((up%3600/60))

        db_ok=$(d '.data.database.healthy'); redis_ok=$(d '.data.redis.healthy')
        [ "$db_ok" = "true" ] && ok "PostgreSQL: healthy, миграция $(d '.data.database.schema_version')$([ "$(d '.data.database.schema_dirty')" = "true" ] && echo ' (DIRTY!)'), соединения $(d '.data.database.in_use')/$(d '.data.database.max_open')" \
            || { bad "PostgreSQL: UNHEALTHY"; FAILURES=$((FAILURES+1)); }
        [ "$redis_ok" = "true" ] && ok "Redis: healthy" || { bad "Redis: UNHEALTHY"; FAILURES=$((FAILURES+1)); }

        printf "  %bОчереди:%b high=%s med=%s low=%s compile=%s dead_letter=%s\n" "$B" "$N" \
            "$(d '.data.queues.high')" "$(d '.data.queues.medium')" "$(d '.data.queues.low')" \
            "$(d '.data.queues.compile')" "$(d '.data.queues.dead_letter')"
        dl=$(d '.data.queues.dead_letter')
        [ -n "$dl" ] && [ "$dl" -gt 0 ] 2>/dev/null && warn "dead-letter не пуст ($dl) — повреждённые задачи, смотри логи worker'а"

        printf "  %bМатчи:%b %s\n" "$B" "$N" "$(echo "$status" | jq -rc '.data.matches.by_status // {}')"
        printf "  %bПрограммы:%b %s\n" "$B" "$N" "$(echo "$status" | jq -rc '.data.programs // {}')"

        op=$(d '.data.outbox.pending'); oe=$(d '.data.outbox.errors')
        printf "  %bOutbox:%b pending=%s errors=%s done_24h=%s\n" "$B" "$N" \
            "${op:-0}" "${oe:-0}" "$(d '.data.outbox.done_last_24h')"
        [ -n "$oe" ] && [ "$oe" -gt 0 ] 2>/dev/null && { bad "outbox errors > 0 — рейтинги могли не примениться!"; FAILURES=$((FAILURES+1)); }

        printf "  %bWebSocket:%b клиентов=%s каналов=%s\n" "$B" "$N" \
            "$(d '.data.websocket.total_clients')" "$(d '.data.websocket.tournaments')"
    fi
fi

# -------------------------------------------------------------------- итог
hdr "── Итог ──────────────────────────────────────────────────"
if [ "$FAILURES" -eq 0 ]; then
    ok "Критичных проблем не найдено"
else
    bad "Проблем: $FAILURES (см. выше)"
fi
exit $((FAILURES > 0 ? 1 : 0))
