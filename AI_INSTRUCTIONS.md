# Project: TagNote (tsn)

## Critical Rules

### NEVER destroy Docker volumes
**NEVER run `docker compose down -v` or `docker volume rm` in this project.**
The `-v` flag deletes the data volume containing the SQLite database with all user notes.
Use `docker compose down` (without `-v`) to stop containers safely.
If you need a fresh database for testing, use a separate compose project or a temporary volume — never touch the main one.

### Docker testing
When testing with Docker, always use `docker compose build && docker compose up -d` to rebuild and restart.
To check logs: `docker compose logs --tail=50`.
The dev server runs on port 3777 (mapped to container port 3000).
Test user credentials (when TAGNOTE_TEST_MODE=1): `test@test.com` / `testpass123`.

### NEVER call `go` directly
**NEVER run `go build`, `go run`, `go vet`, `go test`, `go mod tidy`, or any `go` command directly on the host.**
Always use Docker to build and test:

```bash
# Install/update dependencies
docker run --rm -v "$(pwd)":/app -w /app golang:1.22-alpine go mod tidy

# Build specific packages
docker run --rm -v "$(pwd)":/app -w /app golang:1.22-alpine go build ./internal/...

# Run tests
docker run --rm -v "$(pwd)":/app -w /app golang:1.22-alpine go test ./...

# Full Docker build (includes image generation)
docker compose build

# Or use release script
./release/build.sh

# Run locally for development
docker compose up
```

**NEVER suggest `go run ./cmd/...` — always use `docker compose up` to run locally.**

### NEVER install Node dependencies on the host
**NEVER run `npm install`, `npm ci`, `npx playwright install`, or other dependency-installing Node commands directly on the host.**
Use Docker for Node tooling too, and run the container with the host UID/GID so generated files are not root-owned:

```bash
# Install/update Node dependencies
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/.npm -v "$(pwd)":/app -w /app node:22-alpine npm install

# Run Playwright tests after the app is already running
docker run --rm --user "$(id -u):$(id -g)" -e npm_config_cache=/tmp/.npm -v "$(pwd)":/app -w /app --network host mcr.microsoft.com/playwright:v1.60.0-noble npm run test:e2e
```

### Releasing a new version
When asked to "release a new version vX.Y.Z":
1. Update `CHANGELOG.md` with a new entry for the version (review commits since last release, add under Added/Changed/Fixed sections, update comparison links at the bottom)
2. Commit the changelog update and tag with `vX.Y.Z`
3. Run `./release/deploy.sh vX.Y.Z`

## Architecture

- **Stack**: Go 1.22 + Fiber v2, SQLite (pure-Go), Vanilla JS SPA, EasyMDE
- **Entry point**: `cmd/tagnote-server/main.go`
- **Layers**: handler → service → repo (SQLite)
- **Frontend**: embedded in `web/` (no build step)
- **Auth**: JWT + bcrypt passwords + Google OAuth
- **Data model**: notes (with soft-delete/trash), tags (with importance/urgency priority), user settings

## Key paths

- `internal/handler/` — HTTP route handlers
- `internal/service/` — business logic
- `internal/repo/` — SQLite repository + migrations
- `internal/model/` — domain types
- `web/app.js` — entire SPA frontend
- `web/style.css` — all styles (5 themes)
- `web/index.html` — HTML shell
