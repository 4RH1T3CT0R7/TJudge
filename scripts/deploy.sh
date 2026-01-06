#!/bin/bash
set -euo pipefail

# General Deploy Script for TJudge
# Usage: ./deploy.sh <environment> <version>

ENVIRONMENT="${1:-staging}"
VERSION="${2:-latest}"
DEPLOY_DIR="/opt/tjudge"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

deploy_staging() {
    log_info "Deploying to staging environment..."

    cd "$DEPLOY_DIR"

    # Pull new images
    log_info "Pulling images for version ${VERSION}..."
    VERSION=${VERSION} docker-compose -f docker-compose.staging.yml pull

    # Run migrations
    log_info "Running database migrations..."
    VERSION=${VERSION} docker-compose -f docker-compose.staging.yml run --rm api /app/bin/migrate up

    # Deploy with rolling update
    log_info "Deploying services..."
    VERSION=${VERSION} docker-compose -f docker-compose.staging.yml up -d --remove-orphans

    # Wait for health
    log_info "Waiting for services to be healthy..."
    sleep 10

    # Health check
    if curl -sf http://localhost:8080/health > /dev/null; then
        log_success "Staging deployment successful!"
    else
        log_error "Health check failed!"
        exit 1
    fi
}

deploy_production() {
    log_info "Production deployment should use blue-green strategy"
    log_info "Use: ./blue-green-deploy.sh ${VERSION}"
    exec "${DEPLOY_DIR}/scripts/blue-green-deploy.sh" "${VERSION}"
}

main() {
    log_info "=== TJudge Deployment ==="
    log_info "Environment: ${ENVIRONMENT}"
    log_info "Version: ${VERSION}"
    echo ""

    case "$ENVIRONMENT" in
        staging)
            deploy_staging
            ;;
        production)
            deploy_production
            ;;
        *)
            log_error "Unknown environment: ${ENVIRONMENT}"
            log_info "Usage: $0 <staging|production> <version>"
            exit 1
            ;;
    esac
}

main "$@"
