#!/usr/bin/env bash
# ============================================================
# [LOCAL] First-time Grafana + VictoriaMetrics setup
#
# This script SSHes into the server and:
#   1. Creates monitoring directory structure
#   2. Copies monitoring config files (docker-compose, prometheus.yml, grafana provisioning)
#   3. Creates shared Docker network
#   4. Generates GRAFANA_ADMIN_PASSWORD and saves to .env
#   5. Starts the monitoring stack
#   6. Connects tagnote container to the monitoring network
#
# Prerequisites:
#   - TagNote is already deployed (./release/setup.sh + ./release/deploy.sh done)
#   - SSH key-based auth to deploy@<server> is working
#
# Usage:
#   ./release/first_time_setup_grafana.sh
#
# Runs on: Your local development machine
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/config.sh"

MONITORING_DIR="${PROD_DIR}/monitoring"

header "First-time Grafana + VictoriaMetrics setup"

info "Target: ${DEPLOY_HOST}"
info "Monitoring directory: ${MONITORING_DIR}"

# Verify SSH access
info "Testing SSH connection..."
if ! ssh -o ConnectTimeout=5 "$DEPLOY_HOST" "echo ok" > /dev/null 2>&1; then
    err "Cannot SSH to ${DEPLOY_HOST}"
    exit 1
fi
ok "SSH connection works"

# Check if TagNote is deployed
info "Checking TagNote deployment..."
if ! ssh "$DEPLOY_HOST" "test -f ${PROD_DIR}/docker-compose.yml"; then
    err "TagNote not deployed. Run ./release/setup.sh and ./release/deploy.sh first."
    exit 1
fi
ok "TagNote is deployed"

# Check if monitoring already set up
EXISTING=$(ssh "$DEPLOY_HOST" "test -f ${MONITORING_DIR}/docker-compose.monitoring.yml && echo yes || echo no")
if [ "$EXISTING" = "yes" ]; then
    warn "Monitoring already set up at ${MONITORING_DIR}"
    echo ""
    read -p "  Overwrite config? This will NOT delete data volumes. [y/N] " -r
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        info "Skipping setup. Run ./release/deploy_grafana.sh to update."
        exit 0
    fi
fi

# Step 1: Create directory structure
header "Creating monitoring directory structure"
ssh "$DEPLOY_HOST" "
    mkdir -p ${MONITORING_DIR}/grafana/provisioning/datasources
    mkdir -p ${MONITORING_DIR}/grafana/provisioning/dashboards
    mkdir -p ${MONITORING_DIR}/grafana/dashboards
"
ok "Directories created"

# Step 2: Copy monitoring files
header "Copying monitoring config files"

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

# Step 3: Create Docker network
header "Creating Docker network"
ssh "$DEPLOY_HOST" "
    docker network create tagnote-network 2>/dev/null || echo 'Network already exists'
"
ok "tagnote-network ready"

# Step 4: Generate Grafana password and save to .env
header "Generating Grafana admin password"
GRAFANA_PASSWORD=$(openssl rand -base64 24 | tr -d '/+=')
OPERATIONAL_TOKEN=$(openssl rand -hex 32)
OPERATIONAL_TOKEN_CREATED=0

# Check if GRAFANA_ADMIN_PASSWORD already in .env
EXISTING_PASSWORD=$(ssh "$DEPLOY_HOST" "grep -s '^GRAFANA_ADMIN_PASSWORD=' ${PROD_DIR}/.env | cut -d= -f2 || echo ''")
if [ -n "$EXISTING_PASSWORD" ] && [ "$EXISTING_PASSWORD" != "" ]; then
    info "Using existing GRAFANA_ADMIN_PASSWORD from .env"
    GRAFANA_PASSWORD="$EXISTING_PASSWORD"
else
    # Add to .env
    ssh "$DEPLOY_HOST" "
        if grep -q '^GRAFANA_ADMIN_PASSWORD=' ${PROD_DIR}/.env 2>/dev/null; then
            sed -i 's/^GRAFANA_ADMIN_PASSWORD=.*/GRAFANA_ADMIN_PASSWORD=${GRAFANA_PASSWORD}/' ${PROD_DIR}/.env
        else
            echo 'GRAFANA_ADMIN_PASSWORD=${GRAFANA_PASSWORD}' >> ${PROD_DIR}/.env
        fi
    "
    ok "GRAFANA_ADMIN_PASSWORD saved to ${PROD_DIR}/.env"
fi

EXISTING_OPERATIONAL_TOKEN=$(ssh "$DEPLOY_HOST" "grep -s '^OPERATIONAL_BEARER_TOKEN=' ${PROD_DIR}/.env | cut -d= -f2- || echo ''")
if [ -n "$EXISTING_OPERATIONAL_TOKEN" ]; then
    info "Using existing OPERATIONAL_BEARER_TOKEN from .env"
    OPERATIONAL_TOKEN="$EXISTING_OPERATIONAL_TOKEN"
else
    OPERATIONAL_TOKEN_CREATED=1
    ssh "$DEPLOY_HOST" "
        if grep -q '^OPERATIONAL_BEARER_TOKEN=' ${PROD_DIR}/.env 2>/dev/null; then
            sed -i 's/^OPERATIONAL_BEARER_TOKEN=.*/OPERATIONAL_BEARER_TOKEN=${OPERATIONAL_TOKEN}/' ${PROD_DIR}/.env
        else
            echo 'OPERATIONAL_BEARER_TOKEN=${OPERATIONAL_TOKEN}' >> ${PROD_DIR}/.env
        fi
    "
    ok "OPERATIONAL_BEARER_TOKEN saved to ${PROD_DIR}/.env"
fi

ssh "$DEPLOY_HOST" "
    printf '%s\n' '${OPERATIONAL_TOKEN}' > ${MONITORING_DIR}/operational_token
    chmod 600 ${MONITORING_DIR}/operational_token
"
ok "operational_token created for VictoriaMetrics"

if [ "$OPERATIONAL_TOKEN_CREATED" = "1" ]; then
    ssh "$DEPLOY_HOST" "
        cd ${PROD_DIR}
        docker compose up -d tagnote
    "
    ok "TagNote restarted with OPERATIONAL_BEARER_TOKEN"
fi

# Step 5: Start monitoring stack
header "Starting monitoring stack"
ssh "$DEPLOY_HOST" "
    cd ${MONITORING_DIR}
    export GRAFANA_ADMIN_PASSWORD='${GRAFANA_PASSWORD}'
    export TAGNOTE_DOMAIN='${TAGNOTE_DOMAIN}'
    docker compose -f docker-compose.monitoring.yml up -d
"
ok "VictoriaMetrics and Grafana started"

# Step 6: Connect TagNote and Caddy to monitoring network
header "Connecting TagNote and Caddy to monitoring network"
ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    TAGNOTE_CONTAINER=\$(docker compose ps -q tagnote 2>/dev/null || echo '')
    if [ -n \"\$TAGNOTE_CONTAINER\" ]; then
        docker network connect tagnote-network \$TAGNOTE_CONTAINER 2>/dev/null || echo 'tagnote already connected'
    else
        echo 'Warning: tagnote container not running'
    fi

    CADDY_CONTAINER=\$(docker compose ps -q caddy 2>/dev/null || echo '')
    if [ -n \"\$CADDY_CONTAINER\" ]; then
        docker network connect tagnote-network \$CADDY_CONTAINER 2>/dev/null || echo 'caddy already connected'
    else
        echo 'Warning: caddy container not running'
    fi
"
ok "Network connected"

# Step 7: Copy updated Caddyfile and restart Caddy
header "Updating Caddyfile and restarting Caddy"
scp "$PROJECT_DIR/Caddyfile" "${DEPLOY_HOST}:${PROD_DIR}/Caddyfile"
ok "Caddyfile updated with /grafana/ route"

ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    docker compose restart caddy
"
ok "Caddy restarted"

# Summary
header "Grafana setup complete"
echo ""
echo "  Grafana URL:      https://${TAGNOTE_DOMAIN}/grafana/"
echo "  Username:         admin"
echo "  Password:         ${GRAFANA_PASSWORD}"
echo ""
echo "  Password saved:   ${PROD_DIR}/.env (GRAFANA_ADMIN_PASSWORD)"
echo ""
echo "  To update dashboards or config later:"
echo "    ./release/deploy_grafana.sh"
echo ""
echo "  To check status:"
echo "    ssh ${DEPLOY_HOST} 'cd ${MONITORING_DIR} && docker compose -f docker-compose.monitoring.yml ps'"
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
    echo "  It may take a moment to start. Try: curl https://${TAGNOTE_DOMAIN}/grafana/api/health"
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
