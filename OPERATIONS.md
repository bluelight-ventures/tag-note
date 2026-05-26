# TagNote Operations Handbook

Production deployment and operations guide for **example.com**.

---

## Architecture Overview

```
Internet → Caddy (TLS termination, :80/:443) → TagNote (:3000) → SQLite + Disk
```

- **Caddy**: Reverse proxy, automatic HTTPS via Let's Encrypt, HTTP→HTTPS redirect, security headers
- **TagNote**: Single Go binary, all frontend assets embedded, SQLite database, file uploads on disk

---

## Server Details

| Item | Value |
|------|-------|
| Provider | Hetzner Cloud CX22 |
| OS | Ubuntu 24.04 LTS |
| Specs | 2 vCPU, 4 GB RAM, 40 GB NVMe |
| Cost | ~$5.49/month |
| Domain | example.com |

---

## Directory Layout

```
/opt/tagnote/
├── docker-compose.yml         # Caddy + TagNote
├── Caddyfile                  # Reverse proxy config
├── .env                       # JWT_SECRET, TAGNOTE_IMAGE, etc.
├── .rollback-image            # Previous image tag (written by deploy.sh)
├── data/
│   ├── tagnote.db             # SQLite database
│   ├── tagnote.db-wal         # WAL journal
│   └── uploads/               # User-uploaded images
├── backups/                   # Local backup snapshots
└── scripts/
    ├── backup.sh              # Daily backup cron script
    └── healthcheck.sh         # Health monitor cron script
```

---

## First-Time Server Setup

### 1. Provision and Harden

```bash
# SSH in as root
ssh root@<VPS_IP>

# Update system
apt update && apt upgrade -y

# Create deploy user
adduser deploy
usermod -aG sudo deploy

# Install Docker
apt install -y docker.io docker-compose-v2 sqlite3
systemctl enable docker
usermod -aG docker deploy

# SSH hardening: edit /etc/ssh/sshd_config
#   PermitRootLogin no
#   PasswordAuthentication no
#   PubkeyAuthentication yes
#   MaxAuthTries 3

# Generate an SSH key pair on your LOCAL machine (skip if you already have one).
# This creates a private key (~/.ssh/id_ed25519) and a public key (~/.ssh/id_ed25519.pub).
# The private key stays on your machine; the public key gets copied to the server.
# ed25519 is the recommended algorithm — faster and more secure than RSA.
#
# Run this on your LOCAL machine (not the server):
#   ssh-keygen -t ed25519 -C "your-email@example.com"
#
# Then copy the public key to the deploy user on the server.
# This adds it to ~deploy/.ssh/authorized_keys so you can SSH in without a password.
# Run this on your LOCAL machine:
ssh-copy-id deploy@<VPS_IP>

systemctl restart ssh

# Firewall
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 443/udp    # HTTP/3
ufw enable

# Brute-force protection
apt install -y fail2ban
systemctl enable fail2ban

# Auto security updates
apt install -y unattended-upgrades
dpkg-reconfigure -plow unattended-upgrades

# Docker log rotation: create /etc/docker/daemon.json
cat > /etc/docker/daemon.json << 'EOF'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
EOF
systemctl restart docker
```

### 2. DNS Records

At your domain registrar, create:

| Type | Name | Value |
|------|------|-------|
| A | @ | `<VPS_IPv4>` |
| A | www | `<VPS_IPv4>` |
| AAAA | @ | `<VPS_IPv6>` |
| AAAA | www | `<VPS_IPv6>` |
| CAA | @ | `0 issue "letsencrypt.org"` |

Set TTL to 300 initially, increase to 3600 once stable.

### 3. Deploy the Application

From your **local machine**, run the setup script. This creates the directory structure,
copies config files (docker-compose.yml, Caddyfile, backup.sh), generates a `.env` with
a random JWT_SECRET, starts Caddy, and installs the backup cron.

```bash
# If DNS is already pointing to the server:
./release/setup.sh

# If DNS is not ready yet, use the IP directly:
DEPLOY_HOST=deploy@<VPS_IP> ./release/setup.sh
```

Then deploy the application:

```bash
./release/deploy.sh
```

Verify:

```bash
curl https://example.com/healthz
```

---

## Deployment Pipeline (SSH Image Transfer)

Images are built locally and transferred to the server via SSH. No container registry is involved.

```
Local machine                          Server (example.com)
-------------------------------------------------------------
1. docker build
   (with version ldflags)

2. docker save | ---- SSH ----> docker load

3. ssh: update .env ----------> TAGNOTE_IMAGE=tagnote:v1.2.0

4. ssh: docker compose ------> restart tagnote container
   up -d tagnote

5. curl /healthz --- verify --> {"version":"v1.2.0","db":true}
```

All release scripts are under `release/` and run from your local machine.
See `release/README.md` for detailed documentation.

### Releasing a New Version

```bash
# 1. Tag
git tag v1.2.0
git push origin v1.2.0

# 2. Deploy to staging first
./release/promote-staging.sh v1.2.0

# 3. Deploy to production
./release/deploy.sh v1.2.0

# 4. Verify
./release/status.sh
```

### Rollback

```bash
# Automatic (uses saved previous image)
./release/rollback.sh

# Manual (specify image)
./release/rollback.sh tagnote:v1.1.0
```

---

## Backups

### What Gets Backed Up

1. **SQLite database** — `/opt/tagnote/data/tagnote.db`
2. **Upload files** — `/opt/tagnote/data/uploads/`

### Schedule

Daily at 3:00 AM UTC via cron. Local retention: 7 days.

### Setup

```bash
# Install the backup cron
chmod +x /opt/tagnote/scripts/backup.sh
sudo crontab -e
# Add: 0 3 * * * /opt/tagnote/scripts/backup.sh >> /var/log/tagnote-backup.log 2>&1
```

### Manual Backup

```bash
/opt/tagnote/scripts/backup.sh
ls -la /opt/tagnote/backups/
```

### Restore from Backup

```bash
cd /opt/tagnote

# Stop the app
docker compose stop tagnote

# Extract backup
tar -xzf backups/tagnote-20260301-030000.tar.gz -C /tmp/restore

# Replace database
cp /tmp/restore/tagnote-*.db data/tagnote.db

# Replace uploads (if needed)
rsync -av /tmp/restore/uploads/ data/uploads/

# Restart
docker compose start tagnote
```

### Remote Backups (Optional)

To push backups to Backblaze B2 (~$0.01/month):

```bash
# Install b2 CLI
pip install b2

# Configure
b2 authorize-account <applicationKeyId> <applicationKey>

# Create bucket
b2 create-bucket tagnote-backups allPrivate

# Uncomment the b2 upload line in scripts/backup.sh
```

---

## Monitoring

### Health & Status Endpoints

```bash
# Liveness + version info
curl https://example.com/healthz
# → {"status":"ok","version":"v1.2.0","build_time":"...","uptime":"48h30m0s","db":true}

# Detailed app metrics
curl https://example.com/status
# → {"version":"v1.2.0","users":12,"notes":347,"tags":89,"db_size_mb":"2.10",...}
```

### Dashboard (from local machine)

```bash
./release/dashboard.sh
```

Shows version, uptime, container status, resource usage, disk, backups,
TLS certificate expiry, and recent errors in one view.

### Server Health Monitor

A cron-based monitor checks `/healthz` every 5 minutes and alerts via webhook on failure.

```bash
# Install from local machine
scp release/server/monitor-setup.sh deploy@example.com:/tmp/
ssh deploy@example.com "sudo bash /tmp/monitor-setup.sh"
```

Configure alerts by setting `TAGNOTE_ALERT_WEBHOOK` in the server's `.env`
to a Slack/Discord/generic webhook URL.

View monitor log: `ssh deploy@example.com "tail -f /var/log/tagnote-monitor.log"`

### Uptime Monitoring (External)

Set up [UptimeRobot](https://uptimerobot.com) (free tier):
- **URL**: `https://example.com/healthz`
- **Interval**: 5 minutes
- **Alert**: Email (or Telegram/Slack webhook)

### Viewing Logs

```bash
# All services
docker compose logs -f

# App only
docker compose logs -f tagnote

# Caddy only (JSON access logs)
docker compose logs -f caddy

# Last 100 lines
docker compose logs --tail 100 tagnote
```

### Disk Usage

```bash
# Check overall disk
df -h /

# Database size
ls -lh /opt/tagnote/data/tagnote.db

# Uploads size
du -sh /opt/tagnote/data/uploads/

# Docker disk usage
docker system df
```

### Resource Usage

```bash
# Container stats (live)
docker stats

# Or check Hetzner Cloud Console for CPU/bandwidth/disk I/O graphs
```

---

## Common Operations

### Restart Services

```bash
cd /opt/tagnote

# Restart everything
docker compose restart

# Restart app only
docker compose restart tagnote

# Full recreate (if config changed)
docker compose up -d
```

### View Active Users / Database Stats

```bash
# Count users
sqlite3 /opt/tagnote/data/tagnote.db "SELECT COUNT(*) FROM users;"

# Count notes
sqlite3 /opt/tagnote/data/tagnote.db "SELECT COUNT(*) FROM subnotes WHERE deleted_at IS NULL;"

# Count notes per user
sqlite3 /opt/tagnote/data/tagnote.db "
  SELECT u.email, COUNT(s.id) as notes
  FROM users u
  LEFT JOIN subnotes s ON s.user_id = u.id AND s.deleted_at IS NULL
  GROUP BY u.id
  ORDER BY notes DESC;
"

# Database file size
ls -lh /opt/tagnote/data/tagnote.db
```

### Run Database Diagnostics

```bash
# Using the built-in diagnose tool
docker compose exec tagnote tagnote-diagnose -db /data/tagnote.db
```

### Update Caddy Configuration

```bash
cd /opt/tagnote
vim Caddyfile
docker compose restart caddy
```

### Check TLS Certificate Status

```bash
# From the server
docker compose exec caddy caddy list-certificates

# From anywhere
curl -vI https://example.com 2>&1 | grep -A 5 "Server certificate"

# Or use SSL Labs: https://www.ssllabs.com/ssltest/analyze.html?d=example.com
```

### Docker Cleanup

```bash
# Remove unused images
docker image prune -f

# Full cleanup (careful — removes stopped containers too)
docker system prune -f
```

---

## Staging Environment

Staging runs on the same server as a separate Docker Compose stack with isolated data.

### Setup

```bash
sudo mkdir -p /opt/tagnote-staging/{data/uploads}
sudo chown -R deploy:deploy /opt/tagnote-staging
cd /opt/tagnote-staging

# Create .env (update VPS_IP with your server's IP)
cat > .env << 'EOF'
JWT_SECRET=staging-secret-not-for-production
TAGNOTE_TEST_MODE=1
TAGNOTE_IMAGE=tagnote:latest
GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
BASE_URL=http://YOUR_VPS_IP:8080
EOF

# Create docker-compose.yml (simplified — no Caddy)
cat > docker-compose.yml << 'YAML'
services:
  tagnote:
    image: ${TAGNOTE_IMAGE:-tagnote:latest}
    restart: unless-stopped
    ports:
      - "8080:3000"
    volumes:
      - ./data:/data
    env_file:
      - .env
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - TAGNOTE_TEST_MODE=${TAGNOTE_TEST_MODE:-1}
YAML

docker compose up -d
```

### Google OAuth on Staging

For Google Sign-In to work on staging:

1. Go to [Google Cloud Console Credentials](https://console.cloud.google.com/apis/credentials)
2. Edit your OAuth 2.0 Client ID
3. Add `http://<VPS_IP>:8080` to **Authorized JavaScript origins**
4. Add `GOOGLE_CLIENT_ID` to staging `.env`

### Access

```
http://<VPS_IP>:8080/app
```

### Deploy to Staging

```bash
# From local machine
./release/promote-staging.sh
```

| | Production | Staging |
|---|---|---|
| Path | `/opt/tagnote/` | `/opt/tagnote-staging/` |
| URL | `https://example.com` | `http://<VPS_IP>:8080` |
| JWT_SECRET | Strong random | Fixed staging value |
| Test user | No | Yes (`test@test.com` / `testpass123`) |
| Deploy | `./release/deploy.sh` | `./release/promote-staging.sh` |
| Backups | Daily | None |

---

## Security Notes

### JWT Secret

The application falls back to `tagnote-dev-secret` if `JWT_SECRET` is not set. In production, **always** set a strong secret:

```bash
openssl rand -hex 32
```

If the JWT secret is compromised, rotate it immediately:

```bash
cd /opt/tagnote
# Generate new secret
NEW_SECRET=$(openssl rand -hex 32)

# Update .env
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$NEW_SECRET/" .env

# Restart (all active sessions will be invalidated)
docker compose restart tagnote
```

### Container Security

- Container runs as non-root user `tagnote` (UID 1001)
- `no-new-privileges` prevents privilege escalation
- Only port 3000 is exposed to the Docker network (not the host)
- Caddy is the only service with host port bindings

### Firewall Rules

Only these ports should be open:

| Port | Protocol | Service |
|------|----------|---------|
| 22 | TCP | SSH |
| 80 | TCP | HTTP (ACME + redirect) |
| 443 | TCP | HTTPS |
| 443 | UDP | HTTP/3 (QUIC) |

Verify: `sudo ufw status`

---

## Troubleshooting

### App Not Responding

```bash
# Check container status
docker compose ps

# Check logs for errors
docker compose logs --tail 50 tagnote

# Check if port is listening
ss -tlnp | grep -E '80|443|3000'

# Restart
docker compose restart
```

### TLS Certificate Issues

```bash
# Check Caddy logs
docker compose logs caddy | grep -i "cert\|tls\|acme"

# Verify DNS is pointing to the server
dig example.com +short

# Force certificate renewal
docker compose restart caddy
```

### Database Locked

SQLite WAL mode handles concurrent reads well, but only one writer at a time. If you see "database is locked" errors:

```bash
# Check for stuck processes
docker compose exec tagnote ls -la /data/tagnote.db*

# WAL checkpoint (flush WAL to main database)
sqlite3 /opt/tagnote/data/tagnote.db "PRAGMA wal_checkpoint(TRUNCATE);"

# Nuclear option: restart
docker compose restart tagnote
```

### Disk Full

```bash
# Check disk usage
df -h /

# Clean Docker resources
docker system prune -f

# Check backup retention
ls -la /opt/tagnote/backups/

# Check logs
du -sh /var/lib/docker/containers/*/
```

---

## Admin Dashboard & Monitoring

### Admin Dashboard

The admin dashboard provides a lightweight user-management panel with overview statistics, registered users list, and audit logs.

**Access:**
- Navigate to `https://example.com/admin`
- You must be logged in with the email matching `ADMIN_EMAIL` env var

**Configuration:**
```bash
# In .env
ADMIN_EMAIL=your-email@example.com
```

If `ADMIN_EMAIL` is not set, admin features are disabled.

**Features:**
- **Overview Cards**: Total users, DAU (today), MAU (30 days), total notes, uptime, DB size
- **Registered Users Table**: Email, display name, created date, verification status, auth methods
- **Audit Logs Table**: Paginated, filterable by user. Shows timestamps, actions, HTTP methods, paths, status codes, IPs

### Prometheus Metrics

The app exposes Prometheus-compatible metrics at `/metrics`:

```bash
curl https://example.com/metrics
```

**Metrics available:**
- `http_requests_total{method,path,status}` — Request counter
- `http_request_duration_seconds{method,path}` — Latency histogram
- `app_registered_users` — Total registered users gauge
- `app_active_users_daily` — DAU gauge
- `app_notes_total` — Total notes gauge
- `app_uptime_seconds` — Process uptime

### Grafana + VictoriaMetrics Stack

For time-series visualization (QPS, latency histograms, etc.), use the monitoring stack.

**Production Deployment:**

1. Copy the monitoring directory to the server:
```bash
scp -r monitoring/ deploy@example.com:/opt/tagnote/
```

2. SSH into the server and start the stack:
```bash
ssh deploy@example.com
cd /opt/tagnote/monitoring

# Create shared Docker network (connects TagNote → VictoriaMetrics)
docker network create tagnote-network || true

# Set environment variables
export GRAFANA_ADMIN_PASSWORD=$(openssl rand -base64 24)
export TAGNOTE_DOMAIN=example.com

# Save password for future reference
echo "GRAFANA_ADMIN_PASSWORD=$GRAFANA_ADMIN_PASSWORD" >> /opt/tagnote/.env

# Start the monitoring stack
docker compose -f docker-compose.monitoring.yml up -d
```

3. Connect the main TagNote container to the monitoring network:
```bash
cd /opt/tagnote
docker network connect tagnote-network $(docker compose ps -q tagnote)
```

4. Restart Caddy to pick up the `/grafana/` proxy:
```bash
docker compose restart caddy
```

**Access Grafana:**
- URL: `https://example.com/grafana/`
- Default credentials: `admin` / `<GRAFANA_ADMIN_PASSWORD>`

**Local Development:**

The dev `docker-compose.yml` includes Grafana + VictoriaMetrics by default:
```bash
docker compose up -d
# Grafana available at http://localhost:3778/grafana/
# Direct access: http://localhost:3001
# Default password: admin
```

**Pre-built Dashboard Panels:**
1. QPS by API Route — `rate(http_requests_total[1m])`
2. Request Latency (p50/p95/p99) — histogram quantiles
3. Error Rate (5xx) — error percentage
4. Requests by Status Code — pie chart
5. Registered Users — gauge
6. Daily Active Users — time series
7. Total Notes — gauge
8. Uptime — gauge

### Audit Logs

All authenticated user actions are logged to the `audit_logs` table:
- User ID, action, HTTP method, path, status code
- Client IP and User-Agent
- Timestamp

Query logs directly:
```bash
sqlite3 /opt/tagnote/data/tagnote.db "
  SELECT created_at, action, path, status
  FROM audit_logs
  ORDER BY created_at DESC
  LIMIT 20;
"
```

---

## Environment Variables Reference

### Core Settings

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `JWT_SECRET` | **Yes** | `tagnote-dev-secret` | HMAC-SHA256 signing key for JWT tokens |
| `TAGNOTE_IMAGE` | **Yes** | `tagnote:latest` | Docker image to run (local image tag) |
| `TAGNOTE_DOMAIN` | **Yes** | — | Domain name (used for `BASE_URL`) |
| `TAGNOTE_TEST_MODE` | No | `0` | Set to `1` to seed a test user |
| `TAGNOTE_ALERT_WEBHOOK` | No | — | Webhook URL for health monitor alerts |
| `GOOGLE_CLIENT_ID` | No | — | Google OAuth client ID for sign-in |
| `BASE_URL` | No | — | Public URL of the app (for OAuth redirects, emails) |

### Email Configuration

Email is required for email verification and password reset. **Priority order:** Amazon SES → SMTP → sendmail.

#### Amazon SES (Recommended for Production)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AWS_SES_ACCESS_KEY` | No | — | AWS access key ID for SES |
| `AWS_SES_SECRET_KEY` | No | — | AWS secret access key for SES |
| `AWS_SES_REGION` | No | `us-east-1` | AWS region (e.g., `us-east-1`, `eu-west-1`) |

**Setup steps:**
1. Create an IAM user with `AmazonSESFullAccess` permission
2. Generate access keys for the IAM user
3. Verify your sender email/domain in the SES console
4. If in sandbox mode, also verify recipient addresses

#### SMTP

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SMTP_HOST` | No | — | SMTP server hostname |
| `SMTP_PORT` | No | `587` | SMTP server port |
| `SMTP_USER` | No | — | SMTP username |
| `SMTP_PASSWORD` | No | — | SMTP password |

#### Common Email Settings

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `EMAIL_FROM` | No | `noreply@example.com` | From address for emails |
| `USE_SENDMAIL` | No | `0` | Set to `1` for system sendmail |

---

## Key File Paths (in repo)

| File | Purpose |
|------|---------|
| `docker-compose.prod.yml` | Production Compose file (Caddy + app) |
| `Caddyfile` | Reverse proxy and TLS config |
| `Dockerfile` | Multi-stage build (Go builder → Alpine runtime) |
| `scripts/backup.sh` | Database + uploads backup script |
| `release/` | All release scripts (see `release/README.md`) |
| `cmd/tagnote-server/main.go` | Server entry point (flags, routes, healthz, status) |
| `internal/handler/handler.go` | API route registration |
| `internal/service/auth.go` | JWT secret loading |
| `internal/repo/sqlite.go` | Database connection (WAL mode) |
