#!/usr/bin/env bash
# ============================================================
# [LOCAL] Check Grafana monitoring stack status
#
# Verifies that Grafana and VictoriaMetrics are running on the server.
#
# Usage:
#   ./release/status_grafana.sh
#
# Runs on: Your local development machine
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/config.sh"

MONITORING_DIR="${PROD_DIR}/monitoring"

header "Grafana Monitoring Status"

info "Target: ${DEPLOY_HOST}"

# Verify SSH access
if ! ssh -o ConnectTimeout=5 "$DEPLOY_HOST" "echo ok" > /dev/null 2>&1; then
    err "Cannot SSH to ${DEPLOY_HOST}"
    exit 1
fi

# Check if monitoring is set up
if ! ssh "$DEPLOY_HOST" "test -f ${MONITORING_DIR}/docker-compose.monitoring.yml" 2>/dev/null; then
    warn "Monitoring not set up. Run ./release/first_time_setup_grafana.sh first."
    exit 1
fi

# Container status
header "Container Status"
ssh "$DEPLOY_HOST" "
    cd ${MONITORING_DIR}
    docker compose -f docker-compose.monitoring.yml ps --format 'table {{.Name}}\t{{.Status}}\t{{.Ports}}'
"

# Health checks
header "Health Checks"

echo -n "Grafana:          "
GRAFANA_STATUS=$(ssh "$DEPLOY_HOST" "curl -s -o /dev/null -w '%{http_code}' http://localhost:3001/grafana/api/health 2>/dev/null || echo 'failed'")
if [ "$GRAFANA_STATUS" = "200" ]; then
    ok "Healthy (HTTP 200)"
else
    warn "HTTP ${GRAFANA_STATUS}"
fi

echo -n "VictoriaMetrics:  "
VM_STATUS=$(ssh "$DEPLOY_HOST" "curl -s -o /dev/null -w '%{http_code}' http://localhost:8428/health 2>/dev/null || echo 'failed'")
if [ "$VM_STATUS" = "200" ]; then
    ok "Healthy (HTTP 200)"
else
    warn "HTTP ${VM_STATUS}"
fi

echo -n "TagNote /metrics: "
METRICS_STATUS=$(ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    OPERATIONAL_TOKEN=\$(grep -s '^OPERATIONAL_BEARER_TOKEN=' ${PROD_DIR}/.env | cut -d= -f2- || true)
    APP_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || true)
    APP_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}' \"\$APP_CONTAINER\" 2>/dev/null | head -n1)
    if [ -n \"\$OPERATIONAL_TOKEN\" ] && [ -n \"\$APP_IP\" ]; then
        curl -s -o /dev/null -w '%{http_code}' -H \"Authorization: Bearer \$OPERATIONAL_TOKEN\" \"http://\$APP_IP:3000/metrics\" 2>/dev/null || echo 'failed'
    else
        echo 'missing-token'
    fi
")
if [ "$METRICS_STATUS" = "200" ]; then
    ok "Healthy (HTTP 200)"
else
    warn "HTTP ${METRICS_STATUS}"
fi

# External access check
header "External Access"
echo -n "Grafana (HTTPS):  "
EXTERNAL_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "https://${TAGNOTE_DOMAIN}/grafana/api/health" 2>/dev/null || echo 'failed')
if [ "$EXTERNAL_STATUS" = "200" ]; then
    ok "Accessible (HTTP 200)"
else
    warn "HTTP ${EXTERNAL_STATUS}"
fi

# Resource usage
header "Resource Usage"
ssh "$DEPLOY_HOST" "
    docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}' \
        \$(cd ${MONITORING_DIR} && docker compose -f docker-compose.monitoring.yml ps -q) 2>/dev/null || echo 'No containers running'
"

# Summary
header "URLs"
echo "  Grafana:     https://${TAGNOTE_DOMAIN}/grafana/"
echo "  Metrics:     private Docker network with OPERATIONAL_BEARER_TOKEN"
echo "  VM (direct): http://<server>:8428"
