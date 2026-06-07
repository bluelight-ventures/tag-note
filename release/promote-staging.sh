#!/usr/bin/env bash
# ============================================================
# [LOCAL] Deploy to staging for validation before production
#
# Usage:
#   ./release/promote-staging.sh           # build and deploy to staging
#   ./release/promote-staging.sh v1.2.3    # explicit version
#   ./release/promote-staging.sh --skip-build  # use existing image
#
# Runs on: Your local development machine
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/config.sh"

SKIP_BUILD=false
VERSION=""

for arg in "$@"; do
    case $arg in
        --skip-build) SKIP_BUILD=true ;;
        *) VERSION="$arg" ;;
    esac
done

if [ -z "$VERSION" ]; then
    VERSION="$(get_version)"
fi

header "Deploying TagNote $VERSION to STAGING"

# Step 1: Build
if [ "$SKIP_BUILD" = false ]; then
    info "Building image..."
    "$SCRIPT_DIR/build.sh" "$VERSION"
fi

# Step 2: Transfer to server
info "Transferring image to server..."
TARBALL="/tmp/tagnote-${VERSION}.tar"
docker save "${IMAGE_NAME}:${VERSION}" -o "$TARBALL"

if command -v pv &> /dev/null; then
    pv "$TARBALL" | ssh "$DEPLOY_HOST" "docker load"
else
    cat "$TARBALL" | ssh "$DEPLOY_HOST" "docker load"
fi
rm -f "$TARBALL"

# Step 3: Update staging and restart
info "Restarting staging..."
ssh "$DEPLOY_HOST" "
    cd ${STAGING_DIR}
    if grep -q '^TAGNOTE_IMAGE=' .env 2>/dev/null; then
        sed -i 's|^TAGNOTE_IMAGE=.*|TAGNOTE_IMAGE=${IMAGE_NAME}:${VERSION}|' .env
    else
        echo 'TAGNOTE_IMAGE=${IMAGE_NAME}:${VERSION}' >> .env
    fi
    if ! grep -q '^OPERATIONAL_BEARER_TOKEN=' .env 2>/dev/null || [ -z \"\$(grep '^OPERATIONAL_BEARER_TOKEN=' .env | cut -d= -f2-)\" ]; then
        echo \"OPERATIONAL_BEARER_TOKEN=$(openssl rand -hex 32)\" >> .env
    fi
    docker compose up -d tagnote
"

# Step 4: Verify
sleep 3
STATUS=$(ssh "$DEPLOY_HOST" "
    cd ${STAGING_DIR}
    OPERATIONAL_TOKEN=\$(grep -s '^OPERATIONAL_BEARER_TOKEN=' ${STAGING_DIR}/.env | cut -d= -f2- || true)
    APP_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || true)
    APP_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}' \"\$APP_CONTAINER\" 2>/dev/null | head -n1)
    if [ -n \"\$OPERATIONAL_TOKEN\" ] && [ -n \"\$APP_IP\" ]; then
        curl -sf -H \"Authorization: Bearer \$OPERATIONAL_TOKEN\" \"http://\$APP_IP:3000/status\" 2>/dev/null || echo '{}'
    else
        echo '{}'
    fi
")
REPORTED_VERSION=$(echo "$STATUS" | grep -o '"version":"[^"]*"' | cut -d'"' -f4 || echo "unknown")

if [ "$REPORTED_VERSION" = "$VERSION" ]; then
    ok "Staging deployment verified!"
else
    warn "Could not verify staging version (got: $REPORTED_VERSION)"
fi

"$SCRIPT_DIR/verify_mcp.sh" staging

# Get server IP for staging URL
SERVER_IP=$(ssh "$DEPLOY_HOST" "hostname -I | awk '{print \$1}'" 2>/dev/null || echo "SERVER_IP")

echo ""
ok "Staging is ready:"
echo "  URL:     http://${SERVER_IP}:8080/app"
echo "  Login:   test@test.com / testpass123"
echo ""
echo "  After validation, deploy to production:"
echo "    ./release/deploy.sh --skip-build $VERSION"
