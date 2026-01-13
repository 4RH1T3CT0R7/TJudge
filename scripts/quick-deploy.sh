#!/bin/bash
set -euo pipefail

# TJudge Quick Deploy Script
# Automatically detects hardware profile and deploys the application
#
# Usage:
#   ./scripts/quick-deploy.sh          # Auto-detect profile
#   ./scripts/quick-deploy.sh weak     # Use weak profile
#   ./scripts/quick-deploy.sh medium   # Use medium profile
#   ./scripts/quick-deploy.sh strong   # Use strong profile

PROFILE="${1:-auto}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

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
    ls -1 config/profiles/*.env 2>/dev/null | xargs -n1 basename | sed 's/.env$//'
    exit 1
fi

log_info "Using profile: $PROFILE ($ENV_FILE)"

# 2. Initialize secrets if needed
if [ ! -d "secrets" ]; then
    log_info "Initializing secrets..."
    if [ -f "./scripts/init-secrets.sh" ]; then
        ./scripts/init-secrets.sh
    else
        mkdir -p secrets
        echo "secret-$(openssl rand -hex 16)" > secrets/db_password.txt
        echo "secret-$(openssl rand -hex 32)" > secrets/jwt_secret.txt
        echo "secret-$(openssl rand -hex 16)" > secrets/redis_password.txt
        log_info "Generated new secrets in ./secrets/"
    fi
fi

# 3. Create data directories
log_info "Creating data directories..."
mkdir -p data/programs backups

# 4. Set HOST_PROGRAMS_PATH for Docker-in-Docker
export HOST_PROGRAMS_PATH="$PROJECT_DIR/data/programs"
log_info "HOST_PROGRAMS_PATH: $HOST_PROGRAMS_PATH"

# 5. Build images
log_info "Building Docker images..."
if command -v docker-compose &>/dev/null; then
    COMPOSE_CMD="docker-compose"
else
    COMPOSE_CMD="docker compose"
fi

$COMPOSE_CMD -f docker-compose.selfhosted.yml --env-file "$ENV_FILE" build --parallel

# 6. Start services
log_info "Starting services..."
$COMPOSE_CMD -f docker-compose.selfhosted.yml --env-file "$ENV_FILE" up -d

# 7. Wait for services
log_info "Waiting for services to be ready..."
RETRIES=30
WAIT_SECONDS=2

for i in $(seq 1 $RETRIES); do
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        break
    fi

    if [ "$i" -eq "$RETRIES" ]; then
        log_error "Health check failed after $RETRIES attempts"
        log_info "Checking container logs..."
        $COMPOSE_CMD -f docker-compose.selfhosted.yml logs --tail=20 api
        exit 1
    fi

    echo -n "."
    sleep $WAIT_SECONDS
done
echo ""

# 8. Show status
log_info "Deployment successful!"
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "  Service URLs"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "  API:        ${GREEN}http://localhost:8080${NC}"
echo -e "  Health:     ${GREEN}http://localhost:8080/health${NC}"
echo -e "  Metrics:    ${GREEN}http://localhost:9090/metrics${NC}"
echo ""

# Check if monitoring is enabled
if $COMPOSE_CMD -f docker-compose.selfhosted.yml ps grafana 2>/dev/null | grep -q "running"; then
    echo -e "  Grafana:    ${GREEN}http://localhost:3000${NC} (admin/admin)"
    echo -e "  Prometheus: ${GREEN}http://localhost:9092${NC}"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "  Container Status"
echo -e "${BLUE}========================================${NC}"
echo ""
$COMPOSE_CMD -f docker-compose.selfhosted.yml ps

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "  Useful Commands"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "  View logs:      $COMPOSE_CMD -f docker-compose.selfhosted.yml logs -f"
echo "  Stop:           $COMPOSE_CMD -f docker-compose.selfhosted.yml down"
echo "  Restart:        $COMPOSE_CMD -f docker-compose.selfhosted.yml restart"
echo "  Backup DB:      make backup"
echo "  List backups:   make backup-list"
echo ""
