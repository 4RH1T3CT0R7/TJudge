#!/bin/bash
set -euo pipefail

# TJudge Quick Deploy (self-hosted)
# Подбирает профиль под железо, готовит секреты и каталоги, собирает и запускает
# docker-compose.selfhosted.yml. Настройки сайта (DOMAIN, BASE_URL, CORS...) - в .env,
# значения профиля перекрывают .env.
#
# Usage:
#   ./scripts/quick-deploy.sh          # Auto-detect profile
#   ./scripts/quick-deploy.sh weak     # Use weak profile
#   ./scripts/quick-deploy.sh medium   # Use medium profile
#   ./scripts/quick-deploy.sh strong   # Use strong profile

PROFILE="${1:-auto}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE_NAME=docker-compose.selfhosted.yml

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[$(date '+%H:%M:%S')] WARNING:${NC} $1"
}

log_error() {
    echo -e "${RED}[$(date '+%H:%M:%S')] ERROR:${NC} $1"
}

cd "$PROJECT_DIR"

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  TJudge Quick Deploy${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 1. Determine profile
if [ "$PROFILE" == "auto" ]; then
    log_info "Auto-detecting hardware profile..."
    ./scripts/detect-profile.sh > /dev/null 2>&1 || true

    if [ -f ".env.profile" ]; then
        PROFILE=$(basename "$(readlink .env.profile)" .env)
        log_info "Detected profile: $PROFILE"
    else
        PROFILE="medium"
        log_warn "Could not detect profile, using default: $PROFILE"
    fi
fi

ENV_FILE="config/profiles/${PROFILE}.env"
if [ ! -f "$ENV_FILE" ]; then
    log_error "Profile not found: $ENV_FILE"
    echo "Available profiles:"
    for f in config/profiles/*.env; do basename "$f" .env; done
    exit 1
fi

log_info "Using profile: $PROFILE ($ENV_FILE)"

# 2. Host paths: абсолютный путь к программам нужен песочницам worker'а,
# GID docker.sock - worker'у без root. значения хоста пишутся в .env один раз,
# оттуда их берут и этот скрипт, и ручной docker compose
touch .env
[ -z "$(tail -c1 .env)" ] || echo >> .env   # без перевода строки строки бы склеились
grep -q '^HOST_PROGRAMS_PATH=' .env || echo "HOST_PROGRAMS_PATH=$PROJECT_DIR/data/programs" >> .env
grep -q '^DOCKER_GID=' .env || echo "DOCKER_GID=${DOCKER_GID:-$(stat -c %g /var/run/docker.sock 2>/dev/null || echo 999)}" >> .env

# явный --env-file отключает автозагрузку .env, поэтому он передаётся первым
ENV_ARGS=(--env-file .env --env-file "$ENV_FILE")
compose() { docker compose -f "$COMPOSE_FILE_NAME" "${ENV_ARGS[@]}" "$@"; }

# 3. Secrets (existing files are kept) and data directories
./scripts/init-secrets.sh
./scripts/prepare-data.sh "$COMPOSE_FILE_NAME"

# 4. Build images
log_info "Building Docker images..."
compose build --parallel

# 5. Start services. POSTGRES_PASSWORD_FILE действует только при создании тома,
# а в старых установках пароль роли шёл из DB_PASSWORD: источник истины - secrets/
log_info "Starting services..."
compose up -d --wait postgres
# shellcheck disable=SC2016 # $POSTGRES_USER раскрывается в контейнере
compose exec -T postgres sh -c 'psql -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres' <<'SQL'
\set pw `cat /run/secrets/db_password`
ALTER ROLE CURRENT_USER PASSWORD :'pw';
SQL
compose up -d

# 6. Wait for services: api не публикуется на хост, проверка изнутри контейнера
log_info "Waiting for services to be ready..."
RETRIES=30
WAIT_SECONDS=2

for i in $(seq 1 $RETRIES); do
    if compose exec -T api wget -qO- http://localhost:8080/health > /dev/null 2>&1; then
        break
    fi

    if [ "$i" -eq "$RETRIES" ]; then
        log_error "Health check failed after $RETRIES attempts"
        log_info "Checking container logs..."
        compose logs --tail=20 api
        exit 1
    fi

    echo -n "."
    sleep $WAIT_SECONDS
done
echo ""

DOMAIN=$(sed -n 's/^DOMAIN=//p' .env 2>/dev/null | tail -1)
DOMAIN="${DOMAIN:-localhost}"

# 7. Show status
log_info "Deployment successful!"
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "  Service URLs"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "  Site:       ${GREEN}http://${DOMAIN}${NC} (https - после выпуска сертификата certbot'ом)"
echo -e "  Metrics:    ${GREEN}http://127.0.0.1:9090/metrics${NC} (api), ${GREEN}:9091${NC} (worker)"
echo ""

echo -e "${BLUE}========================================${NC}"
echo -e "  Container Status"
echo -e "${BLUE}========================================${NC}"
echo ""
compose ps

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "  Useful Commands"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "  Compose:        docker compose -f $COMPOSE_FILE_NAME ${ENV_ARGS[*]} <команда>"
echo "  Backups:        ... --profile backup up -d backup"
echo "  Backup DB:      make backup"
echo "  List backups:   make backup-list"
echo ""
