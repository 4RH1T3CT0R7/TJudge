#!/bin/bash
set -euo pipefail

# TJudge Hardware Profile Detection Script
# Automatically detects system resources and recommends a configuration profile
#
# Usage: ./scripts/detect-profile.sh

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Detect CPU cores
detect_cpu() {
    if command -v nproc &>/dev/null; then
        nproc
    elif command -v sysctl &>/dev/null && sysctl -n hw.ncpu &>/dev/null; then
        sysctl -n hw.ncpu
    elif [ -f /proc/cpuinfo ]; then
        grep -c ^processor /proc/cpuinfo
    else
        echo "2"  # Default fallback
    fi
}

# Detect RAM in GB
detect_ram() {
    if command -v free &>/dev/null; then
        # Linux
        free -g | awk '/^Mem:/{print $2}'
    elif command -v sysctl &>/dev/null && sysctl -n hw.memsize &>/dev/null; then
        # macOS
        echo $(($(sysctl -n hw.memsize) / 1073741824))
    elif [ -f /proc/meminfo ]; then
        # Fallback Linux
        awk '/MemTotal/ {print int($2/1024/1024)}' /proc/meminfo
    else
        echo "4"  # Default fallback
    fi
}

# Detect available disk space in GB
detect_disk() {
    if command -v df &>/dev/null; then
        df -BG "$PROJECT_DIR" 2>/dev/null | awk 'NR==2 {gsub("G",""); print $4}' || echo "20"
    else
        echo "20"
    fi
}

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  TJudge Hardware Profile Detection${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

CPU_CORES=$(detect_cpu)
RAM_GB=$(detect_ram)
DISK_GB=$(detect_disk)

echo -e "Detected Hardware:"
echo -e "  CPU Cores: ${GREEN}${CPU_CORES}${NC}"
echo -e "  RAM:       ${GREEN}${RAM_GB} GB${NC}"
echo -e "  Disk:      ${GREEN}${DISK_GB} GB${NC} (available)"
echo ""

# Determine profile based on resources
if [ "$CPU_CORES" -le 2 ] || [ "$RAM_GB" -le 4 ]; then
    PROFILE="weak"
    PROFILE_DESC="2 CPU cores, 4 GB RAM"
    COLOR=$YELLOW
elif [ "$CPU_CORES" -le 4 ] || [ "$RAM_GB" -le 8 ]; then
    PROFILE="medium"
    PROFILE_DESC="4 CPU cores, 8 GB RAM"
    COLOR=$GREEN
else
    PROFILE="strong"
    PROFILE_DESC="8+ CPU cores, 16+ GB RAM"
    COLOR=$BLUE
fi

echo -e "Recommended Profile: ${COLOR}${PROFILE}${NC} (${PROFILE_DESC})"
echo ""

# Show profile details
PROFILE_FILE="$PROJECT_DIR/config/profiles/${PROFILE}.env"
if [ -f "$PROFILE_FILE" ]; then
    echo -e "Profile Configuration:"
    echo -e "  ${BLUE}WORKER_MIN${NC}=$(grep WORKER_MIN "$PROFILE_FILE" | cut -d= -f2)"
    echo -e "  ${BLUE}WORKER_MAX${NC}=$(grep WORKER_MAX "$PROFILE_FILE" | cut -d= -f2)"
    echo -e "  ${BLUE}EXECUTOR_MEMORY_LIMIT${NC}=$(grep EXECUTOR_MEMORY_LIMIT "$PROFILE_FILE" | cut -d= -f2)"
    echo -e "  ${BLUE}EXECUTOR_CPU_QUOTA${NC}=$(grep EXECUTOR_CPU_QUOTA "$PROFILE_FILE" | cut -d= -f2)"
    echo ""
fi

# Warnings
if [ "$RAM_GB" -le 2 ]; then
    log_warn "Very low RAM detected. Consider upgrading hardware or reducing WORKER_MAX."
fi

if [ "$DISK_GB" -le 10 ]; then
    log_warn "Low disk space. Ensure enough space for programs and backups."
fi

# Create symlink to selected profile
ln -sf "config/profiles/${PROFILE}.env" "$PROJECT_DIR/.env.profile"
log_info "Created symlink: .env.profile -> config/profiles/${PROFILE}.env"

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "  Deployment Commands"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "  Quick deploy with detected profile:"
echo -e "    ${GREEN}make deploy${NC}"
echo ""
echo "  Or manually specify a profile:"
echo -e "    ${GREEN}make deploy-weak${NC}    # For weak hardware"
echo -e "    ${GREEN}make deploy-medium${NC}  # For medium hardware"
echo -e "    ${GREEN}make deploy-strong${NC}  # For strong hardware"
echo ""
echo "  Full docker-compose command:"
echo -e "    ${GREEN}docker-compose -f docker-compose.selfhosted.yml --env-file config/profiles/${PROFILE}.env up -d${NC}"
echo ""
