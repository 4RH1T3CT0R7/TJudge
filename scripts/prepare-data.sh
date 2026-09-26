#!/bin/bash
set -euo pipefail

# Каталоги данных на хосте перед docker compose up:
#   - HOST_PROGRAMS_PATH (по умолчанию ./data/programs) и ./backups создаются
#     и отдаются uid 1000: от него работают api, worker, песочницы и бэкап;
#   - программы из старого named volume programs_data один раз копируются в
#     HOST_PROGRAMS_PATH. раньше prod писал их в volume, а песочницы монтировали
#     пустой каталог хоста. volume не удаляется, это делается руками после проверки.
#     отметка о переносе ставится в сам volume: иначе опустевший каталог (другой
#     путь в .env, несмонтированный диск) молча заполнился бы старыми файлами;
#   - если в БД есть программы, а каталог пуст, скрипт завершается с ошибкой,
#     чтобы api и worker не поднялись без файлов программ.
#
# Usage: ./scripts/prepare-data.sh [compose-файл]   (по умолчанию docker-compose.prod.yml)

cd "$(dirname "$0")/.."
COMPOSE_FILE_ARG="${1:-docker-compose.prod.yml}"
HELPER_IMAGE=alpine:3.24
MARKER=.migrated-from-volume

compose() { docker compose -f "$COMPOSE_FILE_ARG" "$@"; }
die() { echo "prepare-data: $*" >&2; exit 1; }

if [ -z "${HOST_PROGRAMS_PATH:-}" ] && [ -f .env ]; then
    HOST_PROGRAMS_PATH=$(sed -n 's/^HOST_PROGRAMS_PATH=//p' .env | tail -1)
fi
HOST_PROGRAMS_PATH="${HOST_PROGRAMS_PATH:-$PWD/data/programs}"
case "$HOST_PROGRAMS_PATH" in
    /*) ;;
    *) die "HOST_PROGRAMS_PATH должен быть абсолютным путём, сейчас: $HOST_PROGRAMS_PATH" ;;
esac
export HOST_PROGRAMS_PATH

mkdir -p "$HOST_PROGRAMS_PATH" backups

# каталоги программ 0750 от uid 1000, поэтому смотреть в них надо из контейнера
in_programs() { docker run --rm -v "$HOST_PROGRAMS_PATH":/p:ro "$HELPER_IMAGE" sh -c "$1"; }
has_files() { [ -n "$(in_programs "find /p -type f | head -n 1")" ]; }

project=$(basename "$PWD" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-')
project="${COMPOSE_PROJECT_NAME:-$project}"
volume=$(docker volume ls -q \
    --filter label=com.docker.compose.project="$project" \
    --filter label=com.docker.compose.volume=programs_data)

if [ -n "$volume" ] && ! docker run --rm -v "$volume":/from:ro "$HELPER_IMAGE" test -e "/from/$MARKER"; then
    if has_files; then
        die "файлы есть и в volume $volume, и в $HOST_PROGRAMS_PATH: объедините их вручную"
    fi
    echo "prepare-data: перенос программ из volume $volume в $HOST_PROGRAMS_PATH"
    compose stop api worker
    docker run --rm -v "$volume":/from -v "$HOST_PROGRAMS_PATH":/to "$HELPER_IMAGE" sh -euc "
        cp -a /from/. /to/
        [ \"\$(find /from -type f | wc -l)\" = \"\$(find /to -type f | wc -l)\" ]
        chown -R 1000:1000 /to
        touch /from/$MARKER"
    echo "prepare-data: перенесено; после проверки volume удаляется так: docker volume rm $volume"
fi

postgres=$(compose ps -q postgres 2>/dev/null || true)
if [ -n "$postgres" ] && ! has_files; then
    count=$(docker exec "$postgres" psql -U "${DB_USER:-tjudge}" -d "${DB_NAME:-tjudge}" -tAc 'SELECT count(*) FROM programs' 2>/dev/null || echo 0)
    if [ "${count:-0}" -gt 0 ]; then
        die "в БД программ: $count, а $HOST_PROGRAMS_PATH пуст. данные не на месте, запуск прерван"
    fi
fi

for dir in "$HOST_PROGRAMS_PATH" "$PWD/backups"; do
    if [ -z "$(find "$dir" -maxdepth 0 -user 1000)" ]; then
        docker run --rm -v "$dir":/d "$HELPER_IMAGE" chown 1000:1000 /d
    fi
done
