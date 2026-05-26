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
HEALTHZ=$(ssh "$DEPLOY_HOST" "curl -sf ${URL_BASE}/healthz 2>/dev/null || echo 'unreachable'")
echo "  $HEALTHZ"

# Status endpoint
info "App metrics..."
STATUS=$(ssh "$DEPLOY_HOST" "curl -sf ${URL_BASE}/status 2>/dev/null || echo 'unreachable'")
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
