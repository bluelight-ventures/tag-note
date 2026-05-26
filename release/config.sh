#!/usr/bin/env bash
# ============================================================
# TagNote Release Configuration
# Sourced by all release scripts. Do not execute directly.
# ============================================================

# Server connection
DEPLOY_HOST="${DEPLOY_HOST:-deploy@example.com}"
PROD_DIR="/opt/tagnote"
STAGING_DIR="/opt/tagnote-staging"

# Docker image naming
IMAGE_NAME="tagnote"

# Derived version from git
get_version() {
    git describe --tags --always --dirty 2>/dev/null || echo "dev"
}

get_git_commit() {
    git rev-parse --short HEAD 2>/dev/null || echo "unknown"
}

get_build_time() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Color output helpers
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
err()   { echo -e "${RED}[ERR]${NC}   $*"; }
header(){ echo -e "\n${BOLD}${CYAN}=== $* ===${NC}\n"; }
