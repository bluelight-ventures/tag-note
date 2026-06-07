#!/usr/bin/env bash
# ============================================================
# [LOCAL] Build Docker image with version tagging
#
# Usage:
#   ./release/build.sh              # auto-detect version from git
#   ./release/build.sh v1.2.3       # explicit version
#
# Runs on: Your local development machine
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/config.sh"

VERSION="${1:-$(get_version)}"
GIT_COMMIT="$(get_git_commit)"
BUILD_TIME="$(get_build_time)"

header "Building TagNote $VERSION"

info "Version:    $VERSION"
info "Commit:     $GIT_COMMIT"
info "Build time: $BUILD_TIME"

cd "$PROJECT_DIR"

# Build the Docker image for linux/amd64 (server architecture)
docker build \
    --platform linux/amd64 \
    --build-arg VERSION="$VERSION" \
    --build-arg BUILD_TIME="$BUILD_TIME" \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    -t "${IMAGE_NAME}:${VERSION}" \
    -t "${IMAGE_NAME}:latest" \
    .

ok "Built ${IMAGE_NAME}:${VERSION} and ${IMAGE_NAME}:latest"

# Show image size
IMAGE_SIZE=$(docker image inspect "${IMAGE_NAME}:${VERSION}" --format='{{.Size}}' | awk '{printf "%.1f MB", $1/1024/1024}')
info "Image size: $IMAGE_SIZE"

# Smoke test (skip if cross-compiling — amd64 image can't run natively on arm64)
LOCAL_ARCH=$(uname -m)
if [ "$LOCAL_ARCH" = "x86_64" ]; then
    info "Running smoke test..."
    CONTAINER_ID=$(docker run -d --rm \
        -p 13777:3000 \
        -e JWT_SECRET=build-test-secret \
        -e TAGNOTE_TEST_MODE=1 \
        -e OPERATIONAL_BEARER_TOKEN=build-test-operational-token \
        "${IMAGE_NAME}:${VERSION}")

    sleep 2

    STATUS=$(curl -sf \
        -H "Authorization: Bearer build-test-operational-token" \
        http://localhost:13777/status 2>/dev/null || echo '{}')
    REPORTED_VERSION=$(echo "$STATUS" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)

    docker stop "$CONTAINER_ID" > /dev/null 2>&1 || true

    if [ "$REPORTED_VERSION" = "$VERSION" ]; then
        ok "Smoke test passed: /status reports version $REPORTED_VERSION"
    else
        warn "Smoke test: expected version '$VERSION', got '$REPORTED_VERSION'"
        warn "Status response: $STATUS"
    fi
else
    info "Skipping local smoke test (built for amd64, running on $LOCAL_ARCH)"
    info "Smoke test will run on the server after deploy"
fi

echo ""
ok "Build complete. Next steps:"
echo "  Deploy to staging:    ./release/promote-staging.sh"
echo "  Deploy to production: ./release/deploy.sh"
