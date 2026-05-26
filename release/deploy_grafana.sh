#!/usr/bin/env bash
# ============================================================
# [LOCAL] Deploy/update Grafana monitoring stack
#
# This script updates the monitoring stack on the server:
#   1. Copies updated config files (prometheus.yml, dashboards, etc.)
#   2. Restarts the monitoring containers to pick up changes
#
# Use this after:
#   - Updating dashboard JSON
#   - Changing prometheus scrape config
#   - Updating Grafana provisioning
#
# Prerequisites:
#   - Monitoring already set up (./release/first_time_setup_grafana.sh done)
#
# Usage:
#   ./release/deploy_grafana.sh
#
# Runs on: Your local development machine
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/config.sh"

MONITORING_DIR="${PROD_DIR}/monitoring"

header "Deploying Grafana monitoring updates"

info "Target: ${DEPLOY_HOST}"
info "Monitoring directory: ${MONITORING_DIR}"

# Verify SSH access
info "Testing SSH connection..."
if ! ssh -o ConnectTimeout=5 "$DEPLOY_HOST" "echo ok" > /dev/null 2>&1; then
    err "Cannot SSH to ${DEPLOY_HOST}"
    exit 1
fi
ok "SSH connection works"

# Check if monitoring is set up
if ! ssh "$DEPLOY_HOST" "test -f ${MONITORING_DIR}/docker-compose.monitoring.yml"; then
    err "Monitoring not set up. Run ./release/first_time_setup_grafana.sh first."
    exit 1
fi
ok "Monitoring stack exists"

# Step 1: Copy updated config files
header "Copying updated config files"

scp "$PROJECT_DIR/monitoring/docker-compose.monitoring.yml" "${DEPLOY_HOST}:${MONITORING_DIR}/docker-compose.monitoring.yml"
ok "docker-compose.monitoring.yml"

scp "$PROJECT_DIR/monitoring/prometheus.yml" "${DEPLOY_HOST}:${MONITORING_DIR}/prometheus.yml"
ok "prometheus.yml"

scp "$PROJECT_DIR/monitoring/grafana/provisioning/datasources/victoriametrics.yml" \
    "${DEPLOY_HOST}:${MONITORING_DIR}/grafana/provisioning/datasources/victoriametrics.yml"
ok "datasources/victoriametrics.yml"

scp "$PROJECT_DIR/monitoring/grafana/provisioning/dashboards/dashboards.yml" \
    "${DEPLOY_HOST}:${MONITORING_DIR}/grafana/provisioning/dashboards/dashboards.yml"
ok "dashboards/dashboards.yml"

scp "$PROJECT_DIR/monitoring/grafana/dashboards/tagnote.json" \
    "${DEPLOY_HOST}:${MONITORING_DIR}/grafana/dashboards/tagnote.json"
ok "dashboards/tagnote.json"

# Step 2: Get Grafana password from .env
GRAFANA_PASSWORD=$(ssh "$DEPLOY_HOST" "grep -s '^GRAFANA_ADMIN_PASSWORD=' ${PROD_DIR}/.env | cut -d= -f2 || echo 'admin'")

# Step 3: Restart monitoring stack
header "Restarting monitoring stack"
ssh "$DEPLOY_HOST" "
    cd ${MONITORING_DIR}
    export GRAFANA_ADMIN_PASSWORD='${GRAFANA_PASSWORD}'
    export TAGNOTE_DOMAIN='example.com'
    docker compose -f docker-compose.monitoring.yml up -d --force-recreate
"
ok "Monitoring stack restarted"

# Step 4: Ensure tagnote and caddy are connected to network
header "Verifying network connection"
ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    TAGNOTE_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || echo '')
    if [ -n \"\$TAGNOTE_CONTAINER\" ]; then
        docker network connect tagnote-network \$TAGNOTE_CONTAINER 2>/dev/null || echo 'tagnote already connected'
    fi

    CADDY_CONTAINER=\$(docker compose ps -q caddy 2>/dev/null || echo '')
    if [ -n \"\$CADDY_CONTAINER\" ]; then
        docker network connect tagnote-network \$CADDY_CONTAINER 2>/dev/null || echo 'caddy already connected'
    fi
"
ok "Network connection verified"

# Step 5: Show status
header "Monitoring stack status"
ssh "$DEPLOY_HOST" "
    cd ${MONITORING_DIR}
    docker compose -f docker-compose.monitoring.yml ps
"

# Summary
header "Deploy complete"
echo ""
echo "  Grafana URL:      https://example.com/grafana/"
echo "  Username:         admin"
echo "  Password:         (see ${PROD_DIR}/.env)"
echo ""
echo "  Dashboard changes will be picked up automatically."
echo "  Prometheus scrape config changes are now active."
echo ""

# Verification
header "Verifying deployment"
echo "Checking container status..."
ssh "$DEPLOY_HOST" "
    cd ${MONITORING_DIR}
    docker compose -f docker-compose.monitoring.yml ps --format 'table {{.Name}}\t{{.Status}}\t{{.Ports}}'
"
echo ""
echo "Checking Grafana health..."
GRAFANA_STATUS=$(ssh "$DEPLOY_HOST" "curl -s -o /dev/null -w '%{http_code}' http://localhost:3001/grafana/api/health 2>/dev/null || echo 'failed'")
if [ "$GRAFANA_STATUS" = "200" ]; then
    ok "Grafana is healthy (HTTP 200)"
else
    warn "Grafana health check returned: ${GRAFANA_STATUS}"
    echo "  It may take a moment to restart. Try: curl https://example.com/grafana/api/health"
fi

echo ""
echo "Checking VictoriaMetrics..."
VM_STATUS=$(ssh "$DEPLOY_HOST" "curl -s -o /dev/null -w '%{http_code}' http://localhost:8428/health 2>/dev/null || echo 'failed'")
if [ "$VM_STATUS" = "200" ]; then
    ok "VictoriaMetrics is healthy (HTTP 200)"
else
    warn "VictoriaMetrics health check returned: ${VM_STATUS}"
fi

echo ""
ok "Deployment verification complete"
