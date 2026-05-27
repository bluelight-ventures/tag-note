#!/usr/bin/env bash
# ============================================================
# [LOCAL] Rollback to the previous image on the server
#
# Usage:
#   ./release/rollback.sh                    # rollback to previous version
#   ./release/rollback.sh tagnote:v1.0.0     # rollback to specific image
#
# Reads the saved rollback image from /opt/tagnote/.rollback-image
# (written by deploy.sh before each deployment).
#
# Runs on: Your local development machine
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/config.sh"

ROLLBACK_IMAGE="${1:-}"

header "Rolling back TagNote"

if [ -z "$ROLLBACK_IMAGE" ]; then
    info "Reading previous image from server..."
    ROLLBACK_IMAGE=$(ssh "$DEPLOY_HOST" "cat ${PROD_DIR}/.rollback-image 2>/dev/null || echo ''")

    if [ -z "$ROLLBACK_IMAGE" ] || [ "$ROLLBACK_IMAGE" = "unknown" ]; then
        err "No rollback image found. Specify one explicitly:"
        echo "  ./release/rollback.sh tagnote:v1.0.0"
        echo ""
        echo "Available images on server:"
        ssh "$DEPLOY_HOST" "docker images tagnote --format 'table {{.Repository}}:{{.Tag}}\t{{.CreatedAt}}\t{{.Size}}'"
        exit 1
    fi
fi

info "Rolling back to: $ROLLBACK_IMAGE"

# Verify image exists on server
IMAGE_EXISTS=$(ssh "$DEPLOY_HOST" "docker image inspect ${ROLLBACK_IMAGE} > /dev/null 2>&1 && echo yes || echo no")
if [ "$IMAGE_EXISTS" = "no" ]; then
    err "Image ${ROLLBACK_IMAGE} does not exist on the server."
    echo ""
    echo "Available images on server:"
    ssh "$DEPLOY_HOST" "docker images tagnote --format 'table {{.Repository}}:{{.Tag}}\t{{.CreatedAt}}\t{{.Size}}'"
    exit 1
fi

# Get current version before rollback
CURRENT_STATUS=$(ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    OPERATIONAL_TOKEN=\$(grep -s '^OPERATIONAL_BEARER_TOKEN=' ${PROD_DIR}/.env | cut -d= -f2- || true)
    APP_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || true)
    APP_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}' \"\$APP_CONTAINER\" 2>/dev/null | head -n1)
    if [ -n \"\$OPERATIONAL_TOKEN\" ] && [ -n \"\$APP_IP\" ]; then
        curl -sf -H \"Authorization: Bearer \$OPERATIONAL_TOKEN\" \"http://\$APP_IP:3000/status\" 2>/dev/null || echo '{}'
    else
        echo '{}'
    fi
")
CURRENT_VERSION=$(echo "$CURRENT_STATUS" | grep -o '"version":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
info "Current version: $CURRENT_VERSION"

# Restore config files and update .env
info "Restoring configuration files..."
ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}

    # Restore docker-compose.yml if backup exists
    if [ -f .rollback-docker-compose.yml ]; then
        cp -f .rollback-docker-compose.yml docker-compose.yml
        echo 'Restored docker-compose.yml'
    fi

    # Restore Caddyfile if backup exists
    if [ -f .rollback-Caddyfile ]; then
        cp -f .rollback-Caddyfile Caddyfile
        echo 'Restored Caddyfile'
    fi

    # Update image reference
    sed -i 's|^TAGNOTE_IMAGE=.*|TAGNOTE_IMAGE=${ROLLBACK_IMAGE}|' .env
"
ok "Configuration files restored"

# Restart containers
info "Restarting containers..."
ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    docker compose up -d tagnote
    docker compose restart caddy

    # Reconnect to monitoring network if it exists
    if docker network inspect tagnote-network > /dev/null 2>&1; then
        docker network connect tagnote-network \$(docker compose ps -q tagnote) 2>/dev/null || true
        docker network connect tagnote-network \$(docker compose ps -q caddy) 2>/dev/null || true
    fi
"

# Verify
sleep 3
ROLLED_STATUS=$(ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    OPERATIONAL_TOKEN=\$(grep -s '^OPERATIONAL_BEARER_TOKEN=' ${PROD_DIR}/.env | cut -d= -f2- || true)
    APP_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || true)
    APP_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}' \"\$APP_CONTAINER\" 2>/dev/null | head -n1)
    if [ -n \"\$OPERATIONAL_TOKEN\" ] && [ -n \"\$APP_IP\" ]; then
        curl -sf -H \"Authorization: Bearer \$OPERATIONAL_TOKEN\" \"http://\$APP_IP:3000/status\" 2>/dev/null || echo '{}'
    else
        echo '{}'
    fi
")
ROLLED_VERSION=$(echo "$ROLLED_STATUS" | grep -o '"version":"[^"]*"' | cut -d'"' -f4 || echo "unknown")

ok "Rolled back from ${CURRENT_VERSION} to ${ROLLED_VERSION}"
echo "  Check: https://${TAGNOTE_DOMAIN}/healthz"
