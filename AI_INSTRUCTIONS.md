# Agent Instructions

This file is for AI coding agents working in this repository. `AGENTS.md` and
`CLAUDE.md` point here.

## Project

TagNote (`tsn`) is a self-hosted note-taking app built with Go, Fiber, SQLite,
and an embedded vanilla JavaScript frontend.

## Critical Safety Rules

### Never Destroy Docker Volumes

Never run:

```bash
docker compose down -v
docker volume rm
```

The default Docker volume stores the SQLite database and user uploads. Use this
to stop containers safely:

```bash
docker compose down
```

If a fresh database is needed for testing, use a separate Compose project or a
temporary volume.

### Use Docker For Go Tooling

Do not run Go commands directly on the host. Use Docker:

```bash
docker build --target test .
docker compose build
docker compose up -d
```

The Dockerfile `test` stage runs `gofmt -l cmd internal`, `go vet ./...`,
`go test ./...`, and `govulncheck ./...` with the pinned tool version declared
in the Dockerfile.

Run the app locally with Docker Compose, not `go run`:

```bash
docker compose build
docker compose up -d
```

### Use Docker For Node Tooling

Do not install Node dependencies or Playwright browsers directly on the host.
Use Docker and run with the host UID/GID so generated files are not root-owned:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/.npm \
  -v "$(pwd)":/app \
  -w /app \
  node:22-alpine \
  npm install

docker run --rm \
  --user "$(id -u):$(id -g)" \
  --network host \
  -e npm_config_cache=/tmp/.npm \
  -v "$(pwd)":/app \
  -w /app \
  mcr.microsoft.com/playwright:v1.60.0-noble \
  npm run test:e2e
```

## Dependency Management

Use reproducible, pinned dependency versions. Do not use dynamic dependency
selection such as `@latest`, floating Docker tags, or unbounded version ranges
for project builds, CI, release scripts, or docs.

For Go dependencies:

- Add or update modules with Dockerized `go get module@version`.
- Commit the resulting `go.mod` and `go.sum` changes.
- Prefer the latest stable patched release that fixes the issue or provides the
  needed feature.
- Do not install Go tools on the host.

For test tools:

- Keep tool versions pinned in Docker-controlled paths.
- The Dockerfile `test` stage is the canonical backend quality gate.
- Do not run one-off `govulncheck@latest` commands when updating dependencies;
  update the Dockerfile pin intentionally and run `docker build --target test .`.

For npm dependencies:

- Use lockfile-based installs, including `npm ci` in CI and containers.
- Keep `package-lock.json` committed when npm dependencies change.

For automated updates:

- Prefer Dependabot or an equivalent reviewed PR workflow over dynamic runtime
  dependency resolution.
- Dependency update PRs must pass `docker build --target test .`, Docker image
  build, and E2E tests when relevant.

## Local Testing

Rebuild and restart the local stack:

```bash
docker compose build
docker compose up -d
```

Check logs:

```bash
docker compose logs --tail=50
```

The app runs on port `3777`, mapped to container port `3000`.

When `TAGNOTE_TEST_MODE=1`, the test account is:

| Field | Value |
| --- | --- |
| Email | `test@test.com` |
| Password | `testpass123` |

## Release Instructions

When asked to release `vX.Y.Z`:

1. Review commits since the last release.
2. Update `CHANGELOG.md` with `Added`, `Changed`, and `Fixed` entries.
3. Update changelog comparison links.
4. Commit the changelog update.
5. Tag the commit with `vX.Y.Z`.
6. Run:

```bash
./release/deploy.sh vX.Y.Z
```

## Architecture

- Stack: Go 1.26, Fiber v2, SQLite via pure-Go driver, vanilla JavaScript, EasyMDE.
- Entry point: `cmd/tagnote-server/main.go`.
- Layers: handler -> service -> repository.
- Frontend: embedded files in `web/`.
- Auth: JWT, bcrypt passwords, magic links, Google OAuth.
- Data: users, notes, tags, settings, trash, uploads, and audit logs.

## Key Paths

| Path | Purpose |
| --- | --- |
| `cmd/tagnote-server/` | HTTP server entry point. |
| `cmd/tagnote-*` | CLI tools. |
| `internal/handler/` | HTTP handlers and route behavior. |
| `internal/service/` | Business logic. |
| `internal/repo/` | SQLite repository and migrations. |
| `internal/model/` | Domain types and request/response structs. |
| `internal/admin/` | Admin dashboard, metrics, and audit helpers. |
| `web/app.js` | Main SPA behavior. |
| `web/style.css` | App and landing page styles. |
| `web/index.html` | App HTML shell. |
| `monitoring/` | Grafana and VictoriaMetrics configuration. |
| `release/` | Build, deploy, rollback, and status scripts. |
