#!/bin/bash
set -euo pipefail

# General Deploy Script for TJudge
# Usage: ./deploy.sh <environment> <version>

ENVIRONMENT="${1:-staging}"
VERSION="${2:-latest}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_DIR="$(dirname "$SCRIPT_DIR")"

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

# Cleanup unused Docker images and build cache after a successful deployment.
# We avoid `docker image prune -a` because the tjudge-executor image is
# spawned on-demand by the worker and has no long-running container
# holding its reference -- a blanket prune would delete it and break
# match execution until the next pull.
#
# Instead we retain the N most recent tags per tjudge-* repository.
# Override retention count via TJUDGE_IMAGE_KEEP (default: 3).
cleanup_old_images() {
    local keep="${TJUDGE_IMAGE_KEEP:-3}"
    local repos repo tag old_tags

    log_info "Cleaning up unused Docker resources (keeping ${keep} most recent tjudge-* tags)..."

    # Dangling (untagged) images -- always safe.
    if ! docker image prune -f >/dev/null; then
        log_warning "docker image prune -f failed -- continuing"
    fi

    # Tag-based retention for tjudge-* repositories.
    repos=$(docker images --format '{{.Repository}}' 2>/dev/null \
        | grep -E '(^|/)tjudge-(api|worker|executor|migrate|cli)$' \
        | sort -u) || repos=""

    for repo in $repos; do
        old_tags=$(docker images "$repo" --format '{{.CreatedAt}}|{{.Tag}}' 2>/dev/null \
            | grep -v '|<none>$' \
            | sort -r \
            | awk -F'|' -v keep="$keep" 'NR>keep {print $2}') || old_tags=""

        for tag in $old_tags; do
            log_info "Removing old image: $repo:$tag"
            if ! docker rmi "$repo:$tag" >/dev/null 2>&1; then
                log_warning "  $repo:$tag still in use -- skipped"
            fi
        done
    done

    # Build cache -- safe, affects nothing at runtime.
    if ! docker builder prune -af --filter "until=168h" >/dev/null; then
        log_warning "docker builder prune failed -- continuing"
    fi

    log_success "Docker cleanup completed"
}

deploy_staging() {
    log_info "Deploying to staging environment..."

    cd "$DEPLOY_DIR"

    # Pull new images
    log_info "Pulling images for version ${VERSION}..."
    VERSION=${VERSION} docker compose -f docker-compose.prod.yml pull

    # Deploy with rolling update (migrate service runs automatically via depends_on)
    log_info "Deploying services..."
    VERSION=${VERSION} docker compose -f docker-compose.prod.yml up -d --remove-orphans

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

    cleanup_old_images
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
