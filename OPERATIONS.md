# Operations Guide

This guide covers production deployment, backups, monitoring, and recovery for
TagNote installations.

## Production Architecture

```text
Internet
  -> Caddy on ports 80/443
  -> TagNote container on port 3000
  -> SQLite database and upload directory on persistent disk
```

TagNote is a single Go binary with embedded web assets. Runtime state is stored
in SQLite plus uploaded files.

## Server Requirements

Recommended minimum:

| Resource | Recommendation |
| --- | --- |
| OS | Ubuntu 24.04 LTS or similar Linux server |
| CPU | 1-2 vCPU |
| RAM | 1 GB minimum, 2 GB or more recommended |
| Disk | SSD-backed persistent disk |
| Packages | Docker, Docker Compose plugin, sqlite3, rsync |

## Directory Layout

The release scripts assume this server layout:

```text
/opt/tagnote/
  docker-compose.yml
  docker-compose.prod.yml
  Caddyfile
  .env
  .rollback-image
  data/
    tagnote.db
    tagnote.db-wal
    uploads/
  backups/
  monitoring/
```

## First-Time Setup

1. Create a deploy user.
2. Install Docker and the Docker Compose plugin.
3. Point DNS to the server.
4. Create a production `.env`.
5. Run the setup and deploy scripts from your local machine.

Example:

```bash
DEPLOY_HOST=deploy@notes.example.com ./release/setup.sh
DEPLOY_HOST=deploy@notes.example.com ./release/deploy.sh v1.3.2
```

Required production settings:

```bash
JWT_SECRET=<strong-random-secret>
TAGNOTE_DOMAIN=notes.example.com
BASE_URL=https://notes.example.com
TAGNOTE_ALLOW_DEV_SECRET=0
TAGNOTE_TEST_MODE=0
```

Generate a JWT secret:

```bash
openssl rand -hex 32
```

## Deployment

Release scripts build the Docker image locally, transfer it to the server over
SSH, update `TAGNOTE_IMAGE`, restart the app, and verify `/healthz`.

```bash
./release/promote-staging.sh v1.3.2
./release/deploy.sh v1.3.2
./release/status.sh
```

To deploy an already-built image:

```bash
./release/deploy.sh --skip-build v1.3.2
```

See [release/README.md](release/README.md) for the full release flow.

## Backups

Back up both:

- SQLite database: `/opt/tagnote/data/tagnote.db`
- Uploaded files: `/opt/tagnote/data/uploads/`

Use SQLite's online backup command or stop the app before copying the database.
Do not copy only the main database file while ignoring WAL files unless you use
SQLite's backup API.

Manual backup example on the server:

```bash
cd /opt/tagnote
mkdir -p backups

sqlite3 data/tagnote.db ".backup 'backups/tagnote-$(date +%Y%m%d-%H%M%S).db'"
tar -czf "backups/uploads-$(date +%Y%m%d-%H%M%S).tar.gz" data/uploads
```

Restore example:

```bash
cd /opt/tagnote
docker compose stop tagnote
cp backups/tagnote-YYYYMMDD-HHMMSS.db data/tagnote.db
tar -xzf backups/uploads-YYYYMMDD-HHMMSS.tar.gz -C .
docker compose start tagnote
```

Keep off-server backups for production systems.

## Monitoring

### Built-In Endpoints

```bash
curl https://notes.example.com/healthz
curl http://localhost:3000/status
curl http://localhost:3000/metrics
```

| Endpoint | Purpose |
| --- | --- |
| `/healthz` | Minimal public liveness status. |
| `/status` | App counts and database size. Keep private. |
| `/metrics` | Prometheus-compatible metrics. Keep private. |

`/status` and `/metrics` are protected. Access requires authenticated admin
users using `Authorization: Bearer <jwt>` or callers using
`Authorization: Bearer <OPERATIONAL_BEARER_TOKEN>` when that environment
variable is set. Private-network monitoring jobs must send the operational
bearer token.

### Grafana And VictoriaMetrics

The repository includes a monitoring stack under `monitoring/`.

Local development:

```bash
docker compose up -d
```

The local compose stack defaults `OPERATIONAL_BEARER_TOKEN` to
`dev-operational-token` so VictoriaMetrics can scrape `/metrics`.
If you override that token locally, update `monitoring/operational_token` to
the same value.

Open `http://localhost:3778/grafana/`.

Production setup:

```bash
./release/first_time_setup_grafana.sh
./release/status_grafana.sh
```

Set `GRAFANA_ADMIN_PASSWORD` in production.

### External Uptime Checks

Configure an external monitor against:

```text
https://notes.example.com/healthz
```

Use a 1-5 minute interval and alert on non-2xx responses.

## Common Operations

### View Logs

```bash
cd /opt/tagnote
docker compose logs --tail=100 tagnote
docker compose logs --tail=100 caddy
```

### Restart Services

```bash
cd /opt/tagnote
docker compose restart tagnote
docker compose restart caddy
```

### Check Disk Usage

```bash
df -h /
du -sh /opt/tagnote/data
docker system df
```

### Database Diagnostics

```bash
cd /opt/tagnote
docker compose exec tagnote tagnote-diagnose -db /data/tagnote.db
```

### Roll Back

```bash
./release/rollback.sh
```

Or:

```bash
./release/rollback.sh tagnote:v1.3.1
```

## Admin Dashboard

Set the admin email in `.env`:

```bash
ADMIN_EMAIL=admin@example.com
OPERATIONAL_BEARER_TOKEN=<random-token-for-monitors>
```

Then sign in as that user and open:

```text
https://notes.example.com/admin
```

Admin features include overview statistics, users, and audit logs.

## Email Delivery

Email powers verification, password reset, and magic links. If no provider is
configured, accounts are auto-verified and email flows are disabled.

Provider priority:

1. Amazon SES.
2. SMTP.
3. sendmail.

### Amazon SES

| Variable | Description |
| --- | --- |
| `AWS_SES_ACCESS_KEY` | SES access key ID. |
| `AWS_SES_SECRET_KEY` | SES secret access key. |
| `AWS_SES_REGION` | AWS region, default `us-east-1`. |
| `EMAIL_FROM` | Verified sender address. |

### SMTP

| Variable | Description |
| --- | --- |
| `SMTP_HOST` | SMTP hostname. |
| `SMTP_PORT` | SMTP port, default `587`. |
| `SMTP_USER` | SMTP username. |
| `SMTP_PASSWORD` | SMTP password. |
| `EMAIL_FROM` | Sender address. |

### Sendmail

```bash
USE_SENDMAIL=1
EMAIL_FROM=noreply@example.com
```

## Google OAuth

Set:

```bash
GOOGLE_CLIENT_ID=<client-id>.apps.googleusercontent.com
```

In Google Cloud Console, add the production app origin:

```text
https://notes.example.com
```

## Security Checklist

- Use HTTPS in production.
- Set `JWT_SECRET` to a strong random value.
- Set `TAGNOTE_ALLOW_DEV_SECRET=0` in production.
- Keep `.env`, databases, backups, and uploads out of Git.
- Back up database and uploads regularly.
- Restrict SSH to key-based login where possible.
- Keep the server and Docker images patched.
- Rotate `JWT_SECRET` if it is exposed.

## Troubleshooting

### App Is Not Responding

```bash
cd /opt/tagnote
docker compose ps
docker compose logs --tail=100 tagnote
curl http://127.0.0.1:3000/healthz
```

### TLS Problems

```bash
docker compose logs --tail=100 caddy
dig notes.example.com +short
curl -vI https://notes.example.com
```

### Database Locked

SQLite supports many readers and one writer. If lock errors persist:

```bash
cd /opt/tagnote
docker compose logs --tail=100 tagnote
docker compose restart tagnote
```

### Disk Full

```bash
df -h /
du -sh /opt/tagnote/*
docker system df
```

Prune unused Docker images only after confirming they are not needed for rollback:

```bash
docker image prune -f
```
