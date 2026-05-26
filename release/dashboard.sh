#!/usr/bin/env bash
# ============================================================
# [LOCAL] Rich server dashboard via SSH
#
# Displays: version, uptime, containers, disk, DB stats,
#           backup status, TLS cert expiry, recent errors
#
# Usage:
#   ./release/dashboard.sh
#
# Runs on: Your local development machine (SSHes into server)
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/config.sh"

clear 2>/dev/null || true

echo -e "${BOLD}${CYAN}"
echo "  +-------------------------------------------------------+"
echo "  |               TagNote Server Dashboard                |"
echo "  |               $(date -u +"%Y-%m-%d %H:%M UTC")               |"
echo "  +-------------------------------------------------------+"
echo -e "${NC}"

# Collect all data in a single SSH session for efficiency
DASHBOARD_DATA=$(ssh "$DEPLOY_HOST" bash -s << 'REMOTE_SCRIPT'
    set -e

    echo "===HEALTHZ==="
    curl -sf http://localhost:3000/healthz 2>/dev/null || echo '{"status":"unreachable"}'

    echo "===STATUS==="
    curl -sf http://localhost:3000/status 2>/dev/null || echo '{}'

    echo "===CONTAINERS==="
    cd /opt/tagnote && docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "N/A"

    echo "===DOCKER_STATS==="
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" 2>/dev/null | grep -E "tagnote|caddy" || echo "N/A"

    echo "===DISK==="
    df -h / | tail -1

    echo "===DB_SIZE==="
    ls -lh /opt/tagnote/data/tagnote.db 2>/dev/null | awk '{print $5}' || echo "N/A"

    echo "===UPLOADS==="
    du -sh /opt/tagnote/data/uploads/ 2>/dev/null | awk '{print $1}' || echo "0"

    echo "===BACKUPS==="
    ls -1t /opt/tagnote/backups/*.tar.gz 2>/dev/null | head -3 || echo "none"

    echo "===BACKUP_COUNT==="
    ls -1 /opt/tagnote/backups/*.tar.gz 2>/dev/null | wc -l

    echo "===TLS_EXPIRY==="
    echo | openssl s_client -servername example.com -connect example.com:443 2>/dev/null | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2 || echo "N/A"

    echo "===RECENT_ERRORS==="
    cd /opt/tagnote && docker compose logs --tail=100 tagnote 2>/dev/null | grep -iE "error|fatal|panic" | tail -5 || echo "none"

    echo "===SYSTEM==="
    uptime -p 2>/dev/null || uptime
    echo "---LOAD---"
    cat /proc/loadavg 2>/dev/null | awk '{print $1, $2, $3}' || echo "N/A"
    echo "---MEM---"
    free -h 2>/dev/null | grep Mem | awk '{printf "%s used / %s total (%s free)", $3, $2, $4}' || echo "N/A"

    echo "===END==="
REMOTE_SCRIPT
)

# Parse sections
extract_section() {
    echo "$DASHBOARD_DATA" | sed -n "/^===$1===/,/^===/p" | head -n -1 | tail -n +2
}

HEALTHZ=$(extract_section "HEALTHZ")
APP_STATUS=$(extract_section "STATUS")
CONTAINERS=$(extract_section "CONTAINERS")
DOCKER_STATS=$(extract_section "DOCKER_STATS")
DISK=$(extract_section "DISK")
DB_SIZE=$(extract_section "DB_SIZE")
UPLOADS=$(extract_section "UPLOADS")
BACKUPS=$(extract_section "BACKUPS")
BACKUP_COUNT=$(extract_section "BACKUP_COUNT" | tr -d '[:space:]')
TLS_EXPIRY=$(extract_section "TLS_EXPIRY")
RECENT_ERRORS=$(extract_section "RECENT_ERRORS")
SYSTEM_INFO=$(extract_section "SYSTEM")

# Parse health data
APP_VERSION=$(echo "$HEALTHZ" | grep -o '"version":"[^"]*"' | cut -d'"' -f4 || echo "?")
APP_UPTIME=$(echo "$HEALTHZ" | grep -o '"uptime":"[^"]*"' | cut -d'"' -f4 || echo "?")
DB_OK=$(echo "$HEALTHZ" | grep -o '"db":[a-z]*' | cut -d: -f2 || echo "?")

USER_COUNT=$(echo "$APP_STATUS" | grep -o '"users":[0-9]*' | cut -d: -f2 || echo "?")
NOTE_COUNT=$(echo "$APP_STATUS" | grep -o '"notes":[0-9]*' | cut -d: -f2 || echo "?")
TAG_COUNT=$(echo "$APP_STATUS" | grep -o '"tags":[0-9]*' | cut -d: -f2 || echo "?")

# Display
header "Application"
echo "  Version:     $APP_VERSION"
echo "  Uptime:      $APP_UPTIME"
if [ "$DB_OK" = "true" ]; then
    echo -e "  Database:    ${GREEN}healthy${NC}"
else
    echo -e "  Database:    ${RED}$DB_OK${NC}"
fi
echo "  Users:       $USER_COUNT"
echo "  Notes:       $NOTE_COUNT"
echo "  Tags:        $TAG_COUNT"

header "Containers"
echo "$CONTAINERS" | while IFS= read -r line; do echo "  $line"; done

header "Resource Usage"
echo "$DOCKER_STATS" | while IFS= read -r line; do echo "  $line"; done

header "Storage"
echo "$DISK" | awk '{printf "  System disk:   %s used / %s total (%s)\n", $3, $2, $5}'
echo "  Database:      $DB_SIZE"
echo "  Uploads:       $UPLOADS"

header "Backups ($BACKUP_COUNT total)"
echo "$BACKUPS" | while IFS= read -r line; do
    if [ "$line" != "none" ]; then
        echo "  $(basename "$line")"
    else
        echo "  No backups found"
    fi
done

header "TLS Certificate"
if [ "$TLS_EXPIRY" != "N/A" ] && [ -n "$TLS_EXPIRY" ]; then
    # Try GNU date first, then BSD date
    EXPIRY_EPOCH=$(date -d "$TLS_EXPIRY" +%s 2>/dev/null || date -j -f "%b %d %T %Y %Z" "$TLS_EXPIRY" +%s 2>/dev/null || echo 0)
    NOW_EPOCH=$(date +%s)
    if [ "$EXPIRY_EPOCH" -gt 0 ] 2>/dev/null; then
        DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))
        if [ "$DAYS_LEFT" -gt 30 ]; then
            echo -e "  Expires: $TLS_EXPIRY (${GREEN}${DAYS_LEFT} days${NC})"
        elif [ "$DAYS_LEFT" -gt 7 ]; then
            echo -e "  Expires: $TLS_EXPIRY (${YELLOW}${DAYS_LEFT} days${NC})"
        else
            echo -e "  Expires: $TLS_EXPIRY (${RED}${DAYS_LEFT} days - RENEW NOW${NC})"
        fi
    else
        echo "  Expires: $TLS_EXPIRY"
    fi
else
    echo "  Could not determine TLS expiry"
fi

header "System"
SERVER_UPTIME=$(echo "$SYSTEM_INFO" | head -1)
LOAD=$(echo "$SYSTEM_INFO" | grep -A1 "^---LOAD---" | tail -1)
MEM=$(echo "$SYSTEM_INFO" | grep -A1 "^---MEM---" | tail -1)
echo "  Server:  $SERVER_UPTIME"
echo "  Load:    $LOAD"
echo "  Memory:  $MEM"

header "Recent Errors (last 100 log lines)"
if [ "$RECENT_ERRORS" = "none" ] || [ -z "$RECENT_ERRORS" ]; then
    echo -e "  ${GREEN}No errors found${NC}"
else
    echo "$RECENT_ERRORS" | while IFS= read -r line; do
        echo -e "  ${RED}$line${NC}"
    done
fi

echo ""
echo -e "${CYAN}Dashboard generated at $(date -u +"%Y-%m-%d %H:%M:%S UTC")${NC}"
