#!/bin/bash
set -euo pipefail

# Smoke Test Script for TJudge
# This script runs essential tests to verify the deployment is working

DEPLOY_DIR="/opt/tjudge"
CURRENT_ENV_FILE="${DEPLOY_DIR}/current_env"

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
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Track test results
TESTS_PASSED=0
TESTS_FAILED=0

run_test() {
    local name=$1
    local command=$2

    echo -n "Testing: ${name}... "

    if eval "$command" > /dev/null 2>&1; then
        log_success "passed"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        log_error "failed"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

get_inactive_env() {
    if [ -f "$CURRENT_ENV_FILE" ]; then
        local current=$(cat "$CURRENT_ENV_FILE")
        if [ "$current" == "blue" ]; then
            echo "green"
        else
            echo "blue"
        fi
    else
        echo "blue"
    fi
}

main() {
    local target_env=$(get_inactive_env)
    local api_container="tjudge-api-${target_env}"

    log_info "=== Smoke Tests for ${target_env} environment ==="
    echo ""

    # Test 1: Container is running
    run_test "Container running" "docker ps --filter name=${api_container} --filter status=running | grep -q ${api_container}"

    # Test 2: Container is healthy
    run_test "Container health" "docker inspect --format='{{.State.Health.Status}}' ${api_container} | grep -q healthy"

    # Test 3: Health endpoint responds
    run_test "Health endpoint" "docker exec ${api_container} wget -q --spider http://localhost:8080/health"

    # Test 4: Health endpoint returns 200
    run_test "Health status code" "docker exec ${api_container} wget -qO- --server-response http://localhost:8080/health 2>&1 | grep -q '200 OK'"

    # Test 5: API returns valid JSON
    run_test "Health JSON response" "docker exec ${api_container} wget -qO- http://localhost:8080/health 2>/dev/null | grep -q 'status'"

    # Test 6: Metrics endpoint available
    run_test "Metrics endpoint" "docker exec ${api_container} wget -q --spider http://localhost:9090/metrics" || true

    # Test 7: Check memory usage is reasonable (less than 80% of limit)
    run_test "Memory usage" "
        mem_usage=\$(docker stats ${api_container} --no-stream --format '{{.MemPerc}}' | tr -d '%' | cut -d'.' -f1)
        [ \"\$mem_usage\" -lt 80 ]
    " || true

    # Test 8: Check CPU usage is reasonable
    run_test "CPU usage" "
        cpu_usage=\$(docker stats ${api_container} --no-stream --format '{{.CPUPerc}}' | tr -d '%' | cut -d'.' -f1)
        [ \"\$cpu_usage\" -lt 90 ]
    " || true

    # Test 9: Check no restart loops
    run_test "No restart loops" "
        restarts=\$(docker inspect --format='{{.RestartCount}}' ${api_container})
        [ \"\$restarts\" -lt 3 ]
    "

    # Test 10: Database connectivity (via health check)
    run_test "Database connectivity" "
        health_response=\$(docker exec ${api_container} wget -qO- http://localhost:8080/health 2>/dev/null)
        echo \"\$health_response\" | grep -qE '\"database\".*:.*\"(ok|healthy|connected)\"' || echo \"\$health_response\" | grep -q 'healthy'
    " || true

    # Test 11: Redis connectivity (via health check)
    run_test "Redis connectivity" "
        health_response=\$(docker exec ${api_container} wget -qO- http://localhost:8080/health 2>/dev/null)
        echo \"\$health_response\" | grep -qE '\"cache\".*:.*\"(ok|healthy|connected)\"' || echo \"\$health_response\" | grep -q 'healthy'
    " || true

    # Test 12: Response time check
    run_test "Response time < 1s" "
        start=\$(date +%s%N)
        docker exec ${api_container} wget -qO- http://localhost:8080/health > /dev/null
        end=\$(date +%s%N)
        duration=\$(( (end - start) / 1000000 ))
        [ \"\$duration\" -lt 1000 ]
    "

    echo ""
    log_info "=== Smoke Test Results ==="
    log_info "Passed: ${TESTS_PASSED}"
    log_info "Failed: ${TESTS_FAILED}"

    if [ $TESTS_FAILED -gt 0 ]; then
        log_error "Some smoke tests failed!"
        exit 1
    fi

    log_success "All smoke tests passed!"
    exit 0
}

main "$@"
