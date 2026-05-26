# TagNote Release Process

## Versioning

TagNote uses semantic versioning: `vMAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes (API, database schema)
- **MINOR**: New features
- **PATCH**: Bug fixes, dependency updates

Versions are derived from git tags. If no tag exists on the current commit,
the version falls back to `git describe` output (e.g., `v1.0.0-3-gabcdef`).

## Scripts

| Script | Runs On | Purpose |
|--------|---------|---------|
| `release/build.sh` | Local | Build Docker image with version |
| `release/deploy.sh` | Local | Full pipeline: build + SSH transfer + restart |
| `release/rollback.sh` | Local | Revert to previous version |
| `release/status.sh` | Local | Quick health/disk/container check |
| `release/promote-staging.sh` | Local | Deploy to staging for validation |
| `release/dashboard.sh` | Local | Rich visual server dashboard |
| `release/server/monitor-setup.sh` | Server | Install cron-based health monitor |

## Typical Release Flow

### 1. Tag the release

```bash
git tag v1.2.0
git push origin v1.2.0
```

### 2. Deploy to staging

```bash
./release/promote-staging.sh v1.2.0
# Test at http://<server-ip>:8080/app
# Login: test@test.com / testpass123
```

### 3. Deploy to production

```bash
# Uses the image already built by promote-staging.sh:
./release/deploy.sh --skip-build v1.2.0

# Or build and deploy in one step:
./release/deploy.sh v1.2.0
```

### 4. Verify

```bash
./release/status.sh
# Or for a detailed view:
./release/dashboard.sh
```

### 5. Rollback (if needed)

```bash
# Automatic — uses the image saved before the last deploy
./release/rollback.sh

# Manual — specify an image tag
./release/rollback.sh tagnote:v1.1.0
```

## Deployment Pipeline

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

No container registry is involved. Images are transferred directly via SSH.

## Configuration

Edit `release/config.sh` to change:

| Variable | Default | Purpose |
|----------|---------|---------|
| `DEPLOY_HOST` | `deploy@example.com` | SSH connection string |
| `PROD_DIR` | `/opt/tagnote` | Production directory on server |
| `STAGING_DIR` | `/opt/tagnote-staging` | Staging directory on server |
| `IMAGE_NAME` | `tagnote` | Docker image name |

Override per-invocation via environment: `DEPLOY_HOST=user@other-server ./release/deploy.sh`

## Monitoring

### Health Endpoints

```bash
# Liveness + version
curl https://example.com/healthz
# {"status":"ok","version":"v1.2.0","build_time":"...","uptime":"48h30m0s","db":true}

# Detailed app metrics
curl https://example.com/status
# {"version":"v1.2.0","users":12,"notes":347,"tags":89,"db_size_mb":"2.10",...}
```

### Server Health Monitor

Install a cron job on the server that checks `/healthz` every 5 minutes:

```bash
scp release/server/monitor-setup.sh deploy@example.com:/tmp/
ssh deploy@example.com "sudo bash /tmp/monitor-setup.sh"
```

Configure webhook alerts by setting `TAGNOTE_ALERT_WEBHOOK` in the server's `.env`
(supports Slack, Discord, or any URL accepting a JSON `{"text":"..."}` POST).

### Dashboard

```bash
./release/dashboard.sh
```

Shows version, uptime, container status, resource usage, disk, backups,
TLS certificate expiry, and recent errors — all in one view.
