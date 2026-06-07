# Release Process

This directory contains scripts for building, deploying, rolling back, and
checking TagNote releases.

## Versioning

TagNote uses semantic versioning: `vMAJOR.MINOR.PATCH`.

- `MAJOR`: breaking API, database, or operational changes.
- `MINOR`: backwards-compatible features.
- `PATCH`: bug fixes, documentation, and dependency updates.

Builds derive their version from the explicit script argument or from Git tags.

## Scripts

| Script | Runs On | Purpose |
| --- | --- | --- |
| `release/build.sh` | Local machine | Build a Docker image with version metadata. |
| `release/setup.sh` | Local machine | First-time server directory and config setup. |
| `release/deploy.sh` | Local machine | Build, transfer over SSH, restart, and verify production. |
| `release/promote-staging.sh` | Local machine | Deploy to the staging Compose stack. |
| `release/verify_mcp.sh` | Local machine | Verify the released image contains a working `tagnote-mcp` binary. |
| `release/rollback.sh` | Local machine | Restore the previous or specified image. |
| `release/status.sh` | Local machine | Check production health and container status. |
| `release/dashboard.sh` | Local machine | Show an operator dashboard. |
| `release/first_time_setup_grafana.sh` | Local machine | Install monitoring support. |
| `release/deploy_grafana.sh` | Local machine | Update monitoring configuration. |
| `release/status_grafana.sh` | Local machine | Check monitoring stack status. |
| `release/server/monitor-setup.sh` | Server | Install cron-based health checks. |

## Configuration

Edit `release/config.sh` or override values per command.

| Variable | Default | Purpose |
| --- | --- | --- |
| `DEPLOY_HOST` | `deploy@example.com` | SSH target for production. |
| `TAGNOTE_DOMAIN` | `example.com` | Public production hostname. |
| `PROD_DIR` | `/opt/tagnote` | Production directory on the server. |
| `STAGING_DIR` | `/opt/tagnote-staging` | Staging directory on the server. |
| `IMAGE_NAME` | `tagnote` | Local Docker image name. |

Example override:

```bash
DEPLOY_HOST=deploy@notes.example.com ./release/status.sh
```

## Typical Release Flow

1. Update `CHANGELOG.md`.
2. Commit the release changes.
3. Tag the release.
4. Deploy to staging.
5. Validate the app.
6. Deploy to production.

```bash
git tag v1.3.2
git push origin v1.3.2

./release/promote-staging.sh v1.3.2
./release/deploy.sh --skip-build v1.3.2
./release/status.sh
```

To build and deploy production in one step:

```bash
./release/deploy.sh v1.3.2
```

## Deployment Model

The web app, CLI tools, and HTTP MCP service ship in one Docker image. The
scripts build that image locally and transfer it directly to the server over
SSH. No container registry is required.

```text
local Docker build
  -> docker save
  -> SSH transfer
  -> docker load on server
  -> update TAGNOTE_IMAGE in .env
  -> docker compose up -d tagnote tagnote-mcp caddy
  -> verify /healthz
  -> verify https://mcp.tag-note.com/mcp
```

The production MCP endpoint is:

```text
https://mcp.tag-note.com/mcp
```

The endpoint supports native MCP OAuth 2.1 authorization. MCP clients discover
protected-resource metadata from `/mcp`, dynamically register, open the browser
authorization URL, and receive OAuth bearer tokens after TagNote login and
approval.

To verify the MCP endpoint independently:

```bash
./release/verify_mcp.sh prod
```

## Rollback

Use the image recorded before the last deploy:

```bash
./release/rollback.sh
```

Or specify an image explicitly:

```bash
./release/rollback.sh tagnote:v1.3.1
```

## Verification

```bash
./release/status.sh
./release/dashboard.sh
curl https://notes.example.com/healthz
```

Expected `/healthz` shape:

```json
{"status":"ok"}
```

Detailed version, uptime, and database metrics live on `/status`, which requires
an admin JWT or `OPERATIONAL_BEARER_TOKEN`.

## Monitoring

The release scripts support a small monitoring stack with Grafana and
VictoriaMetrics. See [../OPERATIONS.md](../OPERATIONS.md) for setup details.
