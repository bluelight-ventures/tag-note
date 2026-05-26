#!/usr/bin/env bash
# ============================================================
# [SERVER] Install a cron-based health monitor
#
# This script runs ON THE SERVER (not locally).
# It installs a cron job that checks /healthz every 5 minutes
# and logs failures. Optionally sends alerts via webhook.
#
# Usage (run on server):
#   sudo bash /opt/tagnote/scripts/monitor-setup.sh
#
# Or deploy from local:
#   scp release/server/monitor-setup.sh deploy@example.com:/tmp/
#   ssh deploy@example.com "sudo bash /tmp/monitor-setup.sh"
#
# Runs on: The production server
# ============================================================
set -euo pipefail

MONITOR_SCRIPT="/opt/tagnote/scripts/healthcheck.sh"
LOG_FILE="/var/log/tagnote-monitor.log"
CRON_SCHEDULE="*/5 * * * *"

echo "Installing TagNote health monitor..."

# Create the health check script
cat > "$MONITOR_SCRIPT" << 'HEALTHCHECK'
#!/usr/bin/env bash
# TagNote health check — runs via cron every 5 minutes
# Logs to /var/log/tagnote-monitor.log

HEALTHZ_URL="http://localhost:3000/healthz"
ALERT_WEBHOOK="${TAGNOTE_ALERT_WEBHOOK:-}"
LOG="/var/log/tagnote-monitor.log"
FAILURE_FILE="/tmp/tagnote-healthcheck-failures"

TIMESTAMP=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
RESPONSE=$(curl -sf --max-time 10 "$HEALTHZ_URL" 2>/dev/null || echo "UNREACHABLE")

if echo "$RESPONSE" | grep -q '"status":"ok"'; then
    # Healthy — reset failure counter
    if [ -f "$FAILURE_FILE" ]; then
        PREV_FAILURES=$(cat "$FAILURE_FILE")
        rm -f "$FAILURE_FILE"
        echo "[$TIMESTAMP] RECOVERED after $PREV_FAILURES consecutive failures" >> "$LOG"
        # Send recovery alert
        if [ -n "$ALERT_WEBHOOK" ]; then
            curl -sf -X POST "$ALERT_WEBHOOK" \
                -H "Content-Type: application/json" \
                -d "{\"text\":\"TagNote RECOVERED after ${PREV_FAILURES} failures\"}" \
                > /dev/null 2>&1 || true
        fi
    fi
    # Log healthy check every hour (not every 5 min to avoid noise)
    MINUTE=$(date +%M)
    if [ "$MINUTE" -lt 5 ]; then
        VERSION=$(echo "$RESPONSE" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
        echo "[$TIMESTAMP] OK version=$VERSION" >> "$LOG"
    fi
else
    # Unhealthy — increment failure counter
    FAILURES=1
    if [ -f "$FAILURE_FILE" ]; then
        FAILURES=$(( $(cat "$FAILURE_FILE") + 1 ))
    fi
    echo "$FAILURES" > "$FAILURE_FILE"

    echo "[$TIMESTAMP] FAIL #${FAILURES}: $RESPONSE" >> "$LOG"

    # Alert after 3 consecutive failures (15 minutes)
    if [ "$FAILURES" -eq 3 ] && [ -n "$ALERT_WEBHOOK" ]; then
        # Gather diagnostic info
        CONTAINER_STATUS=$(cd /opt/tagnote && docker compose ps --format "{{.Name}}: {{.Status}}" 2>/dev/null || echo "unknown")
        DISK=$(df -h / | tail -1 | awk '{print $5}')

        curl -sf -X POST "$ALERT_WEBHOOK" \
            -H "Content-Type: application/json" \
            -d "{\"text\":\"TagNote DOWN for 15+ min\nResponse: ${RESPONSE}\nContainers: ${CONTAINER_STATUS}\nDisk: ${DISK}\"}" \
            > /dev/null 2>&1 || true
    fi
fi

# Rotate log if > 10MB
LOG_SIZE=$(stat -c%s "$LOG" 2>/dev/null || stat -f%z "$LOG" 2>/dev/null || echo 0)
if [ "$LOG_SIZE" -gt 10485760 ]; then
    mv "$LOG" "${LOG}.old"
    echo "[$TIMESTAMP] Log rotated" > "$LOG"
fi
HEALTHCHECK

chmod +x "$MONITOR_SCRIPT"
echo "Created $MONITOR_SCRIPT"

# Create log file
touch "$LOG_FILE"
chown deploy:deploy "$LOG_FILE" 2>/dev/null || true
echo "Created $LOG_FILE"

# Install cron job (for the deploy user)
CRON_CMD="${CRON_SCHEDULE} ${MONITOR_SCRIPT}"
EXISTING=$(crontab -u deploy -l 2>/dev/null || echo "")

if echo "$EXISTING" | grep -q "healthcheck.sh"; then
    echo "Cron job already exists — updating..."
    NEW_CRON=$(echo "$EXISTING" | grep -v "healthcheck.sh")
    echo "${NEW_CRON}
${CRON_CMD}" | crontab -u deploy -
else
    echo "${EXISTING}
${CRON_CMD}" | crontab -u deploy -
fi

echo "Installed cron: $CRON_CMD"
echo ""
echo "Monitor installed. To configure alerts, set TAGNOTE_ALERT_WEBHOOK"
echo "in /opt/tagnote/.env to a Slack/Discord/generic webhook URL."
echo ""
echo "View monitor log: tail -f $LOG_FILE"
