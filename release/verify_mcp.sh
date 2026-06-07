#!/usr/bin/env bash
# ============================================================
# [LOCAL] Verify the TagNote MCP endpoint for a released deployment
#
# Usage:
#   ./release/verify_mcp.sh                  # production
#   ./release/verify_mcp.sh prod             # production
#   ./release/verify_mcp.sh staging          # staging
#
# Runs on: Your local development machine (SSHes into server)
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/config.sh"

ENVIRONMENT="${1:-prod}"
case "$ENVIRONMENT" in
    prod|production)
        TARGET_DIR="$PROD_DIR"
        MCP_URL="https://mcp.tag-note.com/mcp"
        ENVIRONMENT="prod"
        ;;
    staging)
        TARGET_DIR="$STAGING_DIR"
        MCP_URL="http://localhost:3779/mcp"
        ;;
    *)
        err "Unknown environment: $ENVIRONMENT"
        echo "Usage: ./release/verify_mcp.sh [prod|staging] [image]"
        exit 1
        ;;
esac

header "Verifying TagNote MCP endpoint ($ENVIRONMENT)"

info "Target: ${DEPLOY_HOST}:${TARGET_DIR}"
info "URL:    ${MCP_URL}"

STATUS=$(ssh "$DEPLOY_HOST" "curl -sS -o /tmp/tagnote-mcp-verify-body -w '%{http_code}' '${MCP_URL}' 2>/tmp/tagnote-mcp-verify-err || true")
if [ "$STATUS" = "401" ] || [ "$STATUS" = "405" ] || [ "$STATUS" = "406" ] || [ "$STATUS" = "415" ]; then
    ok "MCP endpoint is reachable and not publicly usable without a valid MCP request"
else
    err "tagnote-mcp verification failed"
    echo "HTTP status: ${STATUS:-none}"
    ssh "$DEPLOY_HOST" "cat /tmp/tagnote-mcp-verify-err /tmp/tagnote-mcp-verify-body 2>/dev/null || true"
    exit 1
fi

cat <<EOF

Codex production MCP URL:

  https://mcp.tag-note.com/mcp

EOF

ok "MCP verification complete"
