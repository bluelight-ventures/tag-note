#!/usr/bin/env bash
# ============================================================
# [LOCAL] First-time server setup for TagNote
#
# This script SSHes into the server and:
#   1. Creates /opt/tagnote directory structure
#   2. Copies docker-compose.yml, Caddyfile, backup script
#   3. Generates .env with a random JWT_SECRET
#   4. Starts Caddy (but not tagnote — run deploy.sh next)
#
# Prerequisites:
#   - SSH key-based auth to deploy@<server> is working
#   - Docker is installed on the server
#   - DNS for example.com points to the server
#
# Usage:
#   ./release/setup.sh                    # uses DEPLOY_HOST from config.sh
#   DEPLOY_HOST=deploy@1.2.3.4 ./release/setup.sh   # use IP before DNS is ready
#
# Runs on: Your local development machine
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/config.sh"

header "First-time server setup"

info "Target: ${DEPLOY_HOST}"
info "Server directory: ${PROD_DIR}"

# Verify SSH access
info "Testing SSH connection..."
if ! ssh -o ConnectTimeout=5 "$DEPLOY_HOST" "echo ok" > /dev/null 2>&1; then
    err "Cannot SSH to ${DEPLOY_HOST}"
    echo "  Make sure:"
    echo "    1. The server is running"
    echo "    2. SSH key is set up: ssh-copy-id ${DEPLOY_HOST}"
    echo "    3. Or override: DEPLOY_HOST=deploy@<IP> ./release/setup.sh"
    exit 1
fi
ok "SSH connection works"

# Check if already set up
EXISTING=$(ssh "$DEPLOY_HOST" "test -f ${PROD_DIR}/.env && echo yes || echo no")
if [ "$EXISTING" = "yes" ]; then
    warn "${PROD_DIR}/.env already exists on the server."
    echo ""
    read -p "  Overwrite server config? This will NOT delete data. [y/N] " -r
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        info "Skipping setup. Run ./release/deploy.sh to deploy."
        exit 0
    fi
fi

# Step 1: Create directory structure (no sudo — /opt/tagnote should already be owned by deploy)
header "Creating directory structure"
ssh "$DEPLOY_HOST" "
    mkdir -p ${PROD_DIR}/{data/uploads,backups,scripts}
"
ok "Directories created"

# Step 2: Copy config files
header "Copying config files"
scp "$PROJECT_DIR/docker-compose.prod.yml" "${DEPLOY_HOST}:${PROD_DIR}/docker-compose.yml"
ok "docker-compose.yml"

scp "$PROJECT_DIR/Caddyfile" "${DEPLOY_HOST}:${PROD_DIR}/Caddyfile"
ok "Caddyfile"

scp "$PROJECT_DIR/scripts/backup.sh" "${DEPLOY_HOST}:${PROD_DIR}/scripts/backup.sh"
ssh "$DEPLOY_HOST" "chmod +x ${PROD_DIR}/scripts/backup.sh"
ok "backup.sh"

# Step 3: Generate .env
header "Generating .env"
JWT_SECRET=$(openssl rand -hex 32)

ssh "$DEPLOY_HOST" "
    cat > ${PROD_DIR}/.env << 'ENVEOF'
JWT_SECRET=${JWT_SECRET}
TAGNOTE_IMAGE=tagnote:latest
TAGNOTE_DOMAIN=example.com
TAGNOTE_TEST_MODE=0
ADMIN_EMAIL=
GRAFANA_ADMIN_PASSWORD=
# GOOGLE_CLIENT_ID=
# TAGNOTE_ALERT_WEBHOOK=
ENVEOF
    chmod 600 ${PROD_DIR}/.env
"
ok ".env created with generated JWT_SECRET"

# Step 4: Start Caddy only (tagnote needs an image first — run deploy.sh)
header "Starting Caddy"
ssh "$DEPLOY_HOST" "
    cd ${PROD_DIR}
    docker compose up -d caddy 2>&1 || true
"
ok "Caddy started (will get TLS cert once tagnote is running)"

# Step 5: Set up backup cron
header "Setting up daily backup cron"
ssh "$DEPLOY_HOST" "
    EXISTING_CRON=\$(crontab -l 2>/dev/null || echo '')
    if echo \"\$EXISTING_CRON\" | grep -q 'backup.sh'; then
        echo 'Backup cron already exists'
    else
        (echo \"\$EXISTING_CRON\"; echo '0 3 * * * ${PROD_DIR}/scripts/backup.sh >> /var/log/tagnote-backup.log 2>&1') | crontab -
        echo 'Backup cron installed'
    fi
"
ok "Daily backup at 3:00 AM UTC"

# Summary
header "Setup complete"
echo "  Server:    ${DEPLOY_HOST}"
echo "  Directory: ${PROD_DIR}"
echo "  JWT:       (generated — stored in ${PROD_DIR}/.env)"
echo ""
echo "  Next step: deploy the application"
echo ""
echo "    ./release/deploy.sh"
echo ""
echo "  After deploy, verify:"
echo "    curl https://example.com/healthz"
