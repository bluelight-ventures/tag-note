#!/usr/bin/env bash
# ============================================================
# [LOCAL] Full release pipeline: build, transfer via SSH, restart
#
# Usage:
#   ./release/deploy.sh              # auto-detect version, deploy to prod
#   ./release/deploy.sh v1.2.3       # explicit version
#   ./release/deploy.sh --skip-build # transfer existing :latest image
#
# Pipeline:
#   1. Build Docker image (unless --skip-build)
#   2. Save image to tarball
#   3. Transfer via SSH to server
#   4. Load image on server
#   5. Update TAGNOTE_IMAGE in .env
#   6. Restart tagnote container
#   7. Verify health
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

header "Deploying TagNote $VERSION to production"

# Step 1: Build (unless skipped)
if [ "$SKIP_BUILD" = false ]; then
    info "Step 1/7: Building image..."
    "$SCRIPT_DIR/build.sh" "$VERSION"
else
    info "Step 1/7: Skipping build (using existing image)"
    if ! docker image inspect "${IMAGE_NAME}:${VERSION}" > /dev/null 2>&1; then
        err "Image ${IMAGE_NAME}:${VERSION} not found locally. Run build first."
        exit 1
    fi
fi

# Step 2: Save image to tarball
header "Transferring image to server"
info "Step 2/7: Saving image to tarball..."
TARBALL="/tmp/tagnote-${VERSION}.tar"
docker save "${IMAGE_NAME}:${VERSION}" -o "$TARBALL"
TARBALL_SIZE=$(ls -lh "$TARBALL" | awk '{print $5}')
info "Tarball size: $TARBALL_SIZE"

# Step 3: Transfer via SSH (with progress if pv available)
info "Step 3/7: Transferring to ${DEPLOY_HOST}..."
TRANSFER_START=$(date +%s)
if command -v pv &> /dev/null; then
    pv "$TARBALL" | ssh "$DEPLOY_HOST" "docker load"
else
    cat "$TARBALL" | ssh "$DEPLOY_HOST" "docker load"
fi
TRANSFER_END=$(date +%s)
TRANSFER_TIME=$((TRANSFER_END - TRANSFER_START))
ok "Transfer completed in ${TRANSFER_TIME}s"

# Clean up local tarball
rm -f "$TARBALL"

# Step 4: Tag the image on server
info "Step 4/7: Tagging image on server..."
ssh "$DEPLOY_HOST" "docker tag ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}:latest"

# Step 5: Save current image and configs for rollback, then update
info "Step 5/7: Updating server configuration..."
ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}

    # Save current image for rollback
    CURRENT=\$(grep '^TAGNOTE_IMAGE=' .env 2>/dev/null | cut -d= -f2 || echo 'unknown')
    echo \"\$CURRENT\" > .rollback-image

    # Backup current docker-compose.yml and Caddyfile for rollback
    cp -f docker-compose.yml .rollback-docker-compose.yml 2>/dev/null || true
    cp -f Caddyfile .rollback-Caddyfile 2>/dev/null || true

    # Update image reference
    if grep -q '^TAGNOTE_IMAGE=' .env 2>/dev/null; then
        sed -i \"s|^TAGNOTE_IMAGE=.*|TAGNOTE_IMAGE=${IMAGE_NAME}:${VERSION}|\" .env
    else
        echo \"TAGNOTE_IMAGE=${IMAGE_NAME}:${VERSION}\" >> .env
    fi

    # Ensure operational status checks can authenticate after the /healthz
    # endpoint was reduced to public liveness only.
    if ! grep -q '^OPERATIONAL_BEARER_TOKEN=' .env 2>/dev/null || [ -z \"\$(grep '^OPERATIONAL_BEARER_TOKEN=' .env | cut -d= -f2-)\" ]; then
        echo \"OPERATIONAL_BEARER_TOKEN=$(openssl rand -hex 32)\" >> .env
    fi
"

# Copy updated docker-compose.yml and Caddyfile to server
scp "$PROJECT_DIR/docker-compose.prod.yml" "${DEPLOY_HOST}:${PROD_DIR}/docker-compose.yml"
scp "$PROJECT_DIR/Caddyfile" "${DEPLOY_HOST}:${PROD_DIR}/Caddyfile"
ok "Server .env updated, configs copied, rollback files saved"

# Step 6: Restart container and reconnect to monitoring network
info "Step 6/7: Restarting tagnote container..."
ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    docker compose up -d tagnote

    # Reconnect to monitoring network if it exists
    if docker network inspect tagnote-network > /dev/null 2>&1; then
        docker network connect tagnote-network \$(docker compose ps -q tagnote) 2>/dev/null || true
        docker network connect tagnote-network \$(docker compose ps -q caddy) 2>/dev/null || true
    fi
"
ok "Container restarted"

# Step 7: Health verification
info "Step 7/7: Verifying deployment..."
sleep 3

MAX_RETRIES=10
for i in $(seq 1 $MAX_RETRIES); do
    STATUS=$(ssh "$DEPLOY_HOST" "
        cd ${PROD_DIR}
        OPERATIONAL_TOKEN=\$(grep -s '^OPERATIONAL_BEARER_TOKEN=' ${PROD_DIR}/.env | cut -d= -f2- || true)
        APP_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || true)
        APP_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}' \"\$APP_CONTAINER\" 2>/dev/null | head -n1)
        if [ -n \"\$OPERATIONAL_TOKEN\" ] && [ -n \"\$APP_IP\" ]; then
            curl -sf -H \"Authorization: Bearer \$OPERATIONAL_TOKEN\" \"http://\$APP_IP:3000/status\" 2>/dev/null || true
        fi
    " 2>/dev/null || echo "")
    if [ -n "$STATUS" ]; then
        REPORTED_VERSION=$(echo "$STATUS" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)

        if [ "$REPORTED_VERSION" = "$VERSION" ]; then
            ok "Deployment verified!"
            echo ""
            echo "  Version:  $REPORTED_VERSION"
            echo "  URL:      https://${TAGNOTE_DOMAIN}"
            echo ""
            ok "Deploy complete."
            exit 0
        fi
    fi
    info "Waiting for server to be ready... ($i/$MAX_RETRIES)"
    sleep 2
done

err "Health check failed after ${MAX_RETRIES} attempts!"
warn "The container may still be starting, or the deploy failed."
warn "Check logs:  ssh ${DEPLOY_HOST} 'cd ${PROD_DIR} && docker compose logs --tail=50 tagnote'"
warn "Rollback:    ./release/rollback.sh"
exit 1
