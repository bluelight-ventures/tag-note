#!/usr/bin/env bash
# ============================================================
# [LOCAL] Quick server health check
#
# Usage:
#   ./release/status.sh              # check production
#   ./release/status.sh staging      # check staging
#
# Runs on: Your local development machine (SSHes into server)
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/config.sh"

ENV="${1:-prod}"
if [ "$ENV" = "staging" ]; then
    TARGET_DIR="$STAGING_DIR"
    URL_BASE="http://localhost:8080"
else
    TARGET_DIR="$PROD_DIR"
    URL_BASE="http://localhost:3000"
fi

header "TagNote Status ($ENV)"

# Health check
info "Health endpoint..."
HEALTHZ=$(ssh "$DEPLOY_HOST" "
    cd ${TARGET_DIR}
    APP_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || true)
    APP_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}' \"\$APP_CONTAINER\" 2>/dev/null | head -n1)
    if [ -n \"\$APP_IP\" ]; then
        curl -sf \"http://\$APP_IP:3000/healthz\" 2>/dev/null || echo 'unreachable'
    else
        echo 'unreachable'
    fi
")
echo "  $HEALTHZ"

# Status endpoint
info "App metrics..."
STATUS=$(ssh "$DEPLOY_HOST" "
    cd ${TARGET_DIR}
    OPERATIONAL_TOKEN=\$(grep -s '^OPERATIONAL_BEARER_TOKEN=' ${TARGET_DIR}/.env | cut -d= -f2- || true)
    APP_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || true)
    APP_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}' \"\$APP_CONTAINER\" 2>/dev/null | head -n1)
    if [ -n \"\$OPERATIONAL_TOKEN\" ] && [ -n \"\$APP_IP\" ]; then
        curl -sf -H \"Authorization: Bearer \$OPERATIONAL_TOKEN\" \"http://\$APP_IP:3000/status\" 2>/dev/null || echo 'unreachable'
    else
        echo 'unreachable'
    fi
")
echo "  $STATUS"

echo ""

# Container status
info "Container status..."
ssh "$DEPLOY_HOST" "cd ${TARGET_DIR} && docker compose ps"

echo ""

# Disk usage
info "Disk usage..."
ssh "$DEPLOY_HOST" "
    echo '  System:'
    df -h / | tail -1 | awk '{printf \"    Used: %s / %s (%s)\\n\", \$3, \$2, \$5}'
    echo '  Database:'
    ls -lh ${TARGET_DIR}/data/tagnote.db 2>/dev/null | awk '{printf \"    Size: %s\\n\", \$5}' || echo '    Not found'
    echo '  Uploads:'
    du -sh ${TARGET_DIR}/data/uploads/ 2>/dev/null | awk '{printf \"    Size: %s\\n\", \$1}' || echo '    Empty'
    echo '  Docker:'
    docker system df --format 'table {{.Type}}\t{{.TotalCount}}\t{{.Size}}\t{{.Reclaimable}}' 2>/dev/null | sed 's/^/    /'
    echo '  Backups:'
    ls -1 ${TARGET_DIR}/backups/*.tar.gz 2>/dev/null | wc -l | awk '{printf \"    Count: %s\\n\", \$1}'
    ls -lht ${TARGET_DIR}/backups/*.tar.gz 2>/dev/null | head -1 | awk '{printf \"    Latest: %s %s %s (%s)\\n\", \$6, \$7, \$8, \$5}' || echo '    None'
"

echo ""

# Recent logs (last 5 lines)
info "Recent logs..."
ssh "$DEPLOY_HOST" "cd ${TARGET_DIR} && docker compose logs --tail=5 tagnote 2>/dev/null" | head -10

echo ""
ok "Status check complete"
