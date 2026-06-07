#!/usr/bin/env bash
# ============================================================
# [LOCAL] Verify the TagNote MCP binary in a released Docker image
#
# Usage:
#   ./release/verify_mcp.sh                  # production, image from .env
#   ./release/verify_mcp.sh prod             # production, image from .env
#   ./release/verify_mcp.sh staging          # staging, image from .env
#   ./release/verify_mcp.sh prod tagnote:v1  # explicit image
#
# Runs on: Your local development machine (SSHes into server)
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/config.sh"

ENVIRONMENT="${1:-prod}"
IMAGE="${2:-}"

case "$ENVIRONMENT" in
    prod|production)
        TARGET_DIR="$PROD_DIR"
        ENVIRONMENT="prod"
        ;;
    staging)
        TARGET_DIR="$STAGING_DIR"
        ;;
    *)
        err "Unknown environment: $ENVIRONMENT"
        echo "Usage: ./release/verify_mcp.sh [prod|staging] [image]"
        exit 1
        ;;
esac

header "Verifying TagNote MCP ($ENVIRONMENT)"

if [ -z "$IMAGE" ]; then
    info "Reading TAGNOTE_IMAGE from ${TARGET_DIR}/.env..."
    IMAGE=$(ssh "$DEPLOY_HOST" "grep -s '^TAGNOTE_IMAGE=' ${TARGET_DIR}/.env | cut -d= -f2- || true")
fi

if [ -z "$IMAGE" ]; then
    IMAGE="${IMAGE_NAME}:latest"
    warn "TAGNOTE_IMAGE not found; falling back to ${IMAGE}"
fi

info "Target: ${DEPLOY_HOST}:${TARGET_DIR}"
info "Image:  ${IMAGE}"

ssh "$DEPLOY_HOST" "docker image inspect '${IMAGE}' >/dev/null"
ok "Image exists on server"

HELP_OUTPUT=$(ssh "$DEPLOY_HOST" "docker run --rm --entrypoint tagnote-mcp '${IMAGE}' -h 2>&1" || true)
if echo "$HELP_OUTPUT" | grep -q -- "-base-url" && echo "$HELP_OUTPUT" | grep -q -- "-read-only"; then
    ok "tagnote-mcp binary starts and exposes expected flags"
else
    err "tagnote-mcp verification failed"
    echo "$HELP_OUTPUT"
    exit 1
fi

cat <<EOF

Codex production MCP command template:

  TAGNOTE_TOKEN=<user-jwt> \\
  docker run --rm -i \\
    --entrypoint tagnote-mcp \\
    -e TAGNOTE_URL=https://${TAGNOTE_DOMAIN} \\
    -e TAGNOTE_TOKEN \\
    ${IMAGE}

EOF

ok "MCP verification complete"
