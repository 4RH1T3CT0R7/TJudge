#!/bin/bash
set -euo pipefail

# Blue-Green Deployment Script for TJudge
# Usage: ./blue-green-deploy.sh <version>

VERSION="${1:-latest}"
DEPLOY_DIR="/opt/tjudge"
COMPOSE_DIR="${DEPLOY_DIR}/deployments/blue-green"
CURRENT_ENV_FILE="${DEPLOY_DIR}/current_env"
NGINX_CONF_DIR="/etc/nginx/conf.d"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Determine current active environment
get_current_env() {
    if [ -f "$CURRENT_ENV_FILE" ]; then
        cat "$CURRENT_ENV_FILE"
    else
        echo "blue"
    fi
}

# Determine the inactive environment
get_inactive_env() {
    local current=$(get_current_env)
    if [ "$current" == "blue" ]; then
        echo "green"
    else
        echo "blue"
    fi
}

# Wait for containers to be healthy
wait_for_health() {
    local service=$1
    local max_attempts=60
    local attempt=1

    log_info "Waiting for $service to become healthy..."

    while [ $attempt -le $max_attempts ]; do
        if docker inspect --format='{{.State.Health.Status}}' "$service" 2>/dev/null | grep -q "healthy"; then
            log_success "$service is healthy"
            return 0
        fi

        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done

    log_error "$service failed to become healthy within timeout"
    return 1
}

# Deploy to the inactive environment
deploy_to_inactive() {
    local target_env=$(get_inactive_env)
    local compose_file="${COMPOSE_DIR}/docker-compose.${target_env}.yml"

    log_info "Deploying version ${VERSION} to ${target_env} environment"

    # Pull new images
    log_info "Pulling new images..."
    VERSION=${VERSION} docker-compose -f "$compose_file" pull

    # Stop and remove old containers in target environment
    log_info "Stopping existing ${target_env} containers..."
    VERSION=${VERSION} docker-compose -f "$compose_file" down --remove-orphans || true

    # Start new containers
    log_info "Starting new ${target_env} containers..."
    VERSION=${VERSION} docker-compose -f "$compose_file" up -d

    # Wait for API to be healthy
    wait_for_health "tjudge-api-${target_env}"

    # Wait for worker to be running (workers might not have health check)
    sleep 5

    log_success "Deployment to ${target_env} completed"
    echo "$target_env"
}

# Run smoke tests against the new environment
run_smoke_tests() {
    local target_env=$1
    local api_host="tjudge-api-${target_env}"

    log_info "Running smoke tests against ${target_env}..."

    # Health check
    if ! docker exec "${api_host}" wget --quiet --tries=3 --spider "http://localhost:8080/health"; then
        log_error "Health check failed"
        return 1
    fi
    log_success "Health check passed"

    # API version check
    local api_version=$(docker exec "${api_host}" wget -qO- "http://localhost:8080/health" 2>/dev/null | grep -o '"version":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    log_info "API version: ${api_version}"

    # Basic API endpoints check
    local endpoints=("/api/v1/tournaments" "/api/v1/games")
    for endpoint in "${endpoints[@]}"; do
        if docker exec "${api_host}" wget --quiet --tries=2 --spider "http://localhost:8080${endpoint}" 2>/dev/null; then
            log_success "Endpoint ${endpoint} responding"
        else
            log_warning "Endpoint ${endpoint} not responding (might require auth)"
        fi
    done

    log_success "Smoke tests passed"
    return 0
}

# Main deployment flow
main() {
    local current_env=$(get_current_env)
    local target_env=$(get_inactive_env)

    log_info "=== Blue-Green Deployment ==="
    log_info "Current active: ${current_env}"
    log_info "Target environment: ${target_env}"
    log_info "Version: ${VERSION}"
    echo ""

    # Step 1: Deploy to inactive environment
    deploy_to_inactive

    # Step 2: Run smoke tests
    if ! run_smoke_tests "$target_env"; then
        log_error "Smoke tests failed. Rolling back..."
        VERSION=${VERSION} docker-compose -f "${COMPOSE_DIR}/docker-compose.${target_env}.yml" down
        exit 1
    fi

    log_success "=== Deployment to ${target_env} successful ==="
    log_info "Run './scripts/switch-traffic.sh' to switch traffic to the new environment"
    log_info "Run './scripts/rollback.sh' to rollback if needed"
}

main "$@"
