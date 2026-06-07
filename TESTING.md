# Testing

This project is tested through Docker so local runs match CI and do not depend on
host-installed Go, Node, or browser packages.

## Local App Test Mode

Test mode creates a built-in user at startup.

```bash
TAGNOTE_TEST_MODE=1 \
ADMIN_EMAIL=test@test.com \
OPERATIONAL_BEARER_TOKEN=e2e-operational-token \
docker compose build

TAGNOTE_TEST_MODE=1 \
ADMIN_EMAIL=test@test.com \
OPERATIONAL_BEARER_TOKEN=e2e-operational-token \
docker compose up -d
```

| Field | Value |
| --- | --- |
| Email | `test@test.com` |
| Password | `testpass123` |

Open `http://localhost:3777/app` and sign in with those credentials.

## Backend Tests

```bash
docker build --target test .
```

The Dockerfile `test` stage checks formatting with `gofmt -l cmd internal`,
runs `go vet ./...`, runs `go test ./...`, and runs `govulncheck ./...` with
the pinned tool version declared in the Dockerfile.

## API Smoke Tests

Start the app in test mode, then obtain a JWT:

```bash
TOKEN=$(curl -s -X POST http://localhost:3777/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@test.com","password":"testpass123"}' | jq -r '.token')
```

Create and list a note:

```bash
curl -X POST http://localhost:3777/api/v1/notes \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"Test note","tags":["test"]}'

curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:3777/api/v1/notes
```

Verify unauthenticated access is rejected:

```bash
curl http://localhost:3777/api/v1/notes
```

Expected response:

```json
{"error":"missing authorization header"}
```

## CLI Smoke Tests

```bash
docker compose exec tagnote tagnote-login
docker compose exec tagnote tagnote-add -t test "Hello from test user"
docker compose exec tagnote tagnote-read -t test
docker compose exec tagnote tagnote-logs -t test
docker compose exec tagnote tagnote-tags
```

For non-interactive use, export a token inside the environment where the CLI runs:

```bash
export TAGNOTE_TOKEN="$TOKEN"
export TAGNOTE_URL=http://localhost:3000
```

## MCP Server Smoke Tests

Build and start the test-mode stack, then obtain a JWT:

```bash
TAGNOTE_TEST_MODE=1 docker compose build
TAGNOTE_TEST_MODE=1 docker compose up -d

TOKEN=$(curl -s -X POST http://localhost:3777/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@test.com","password":"testpass123"}' | jq -r '.token')
```

Run an MCP client or inspector against the local HTTP MCP endpoint:

```bash
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:3779/mcp
```

For automated verification, use a Dockerized MCP client or inspector and confirm
`tagnote_search_notes`, `tagnote_get_note`, and `tagnote_create_note` are
listed. Set `TAGNOTE_MCP_READ_ONLY=1` and confirm write tools are absent.

## Frontend E2E Tests

Playwright tests live in `tests/` and use `E2E_BASE_URL`.

Install or update Node dependencies through Docker:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e npm_config_cache=/tmp/.npm \
  -v "$PWD":/app \
  -w /app \
  node:22-alpine \
  npm install
```

Build and start the E2E app container:

```bash
docker build -t tag-note:e2e .

docker run -d --rm \
  --name tag-note-e2e-local \
  -p 13777:3000 \
  -e JWT_SECRET=e2e-test-secret \
  -e TAGNOTE_TEST_MODE=1 \
  -e ADMIN_EMAIL=test@test.com \
  -e OPERATIONAL_BEARER_TOKEN=e2e-operational-token \
  -e GOOGLE_CLIENT_ID=web-e2e.apps.googleusercontent.com,ios-e2e.apps.googleusercontent.com \
  -e APPLE_CLIENT_ID=com.tag-note.tagnote,com.tag-note.web-e2e \
  -e APPLE_WEB_CLIENT_ID=com.tag-note.web-e2e \
  tag-note:e2e
```

Wait for the server:

```bash
for i in $(seq 1 30); do
  curl -fsS http://127.0.0.1:13777/healthz >/dev/null && break
  sleep 1
done
```

Run Playwright against the isolated E2E container:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e E2E_BASE_URL=http://host.docker.internal:13777 \
  -e npm_config_cache=/tmp/.npm \
  -v "$PWD":/app \
  -w /app \
  mcr.microsoft.com/playwright:v1.60.0-noble \
  npm run test:e2e
```

Stop the E2E app:

```bash
docker stop tag-note-e2e-local
```

E2E artifacts are written to `test-results/` and `playwright-report/`; both are
ignored by Git.

### Remote SSH E2E

Use this when Docker or browser execution should happen through a remote Linux
host. Use a staging or disposable test host, not the production host. The remote
container is isolated from production data because it uses `--rm`, no mounted
app volume, and a temporary container filesystem.

Build and start the remote E2E app:

```bash
export E2E_HOST=deploy@staging.example.com
export REMOTE_REPO=/opt/tag_note

ssh "$E2E_HOST" "cd $REMOTE_REPO && docker build -t tag-note:e2e ."

ssh "$E2E_HOST" "docker rm -f tag-note-e2e-remote >/dev/null 2>&1 || true"

ssh "$E2E_HOST" "docker run -d --rm \
  --name tag-note-e2e-remote \
  -p 127.0.0.1:13777:3000 \
  -e JWT_SECRET=e2e-test-secret \
  -e TAGNOTE_TEST_MODE=1 \
  -e ADMIN_EMAIL=test@test.com \
  -e OPERATIONAL_BEARER_TOKEN=e2e-operational-token \
  -e GOOGLE_CLIENT_ID=web-e2e.apps.googleusercontent.com,ios-e2e.apps.googleusercontent.com \
  -e APPLE_CLIENT_ID=com.tag-note.tagnote,com.tag-note.web-e2e \
  -e APPLE_WEB_CLIENT_ID=com.tag-note.web-e2e \
  tag-note:e2e"

ssh "$E2E_HOST" "for i in \$(seq 1 30); do \
  curl -fsS http://127.0.0.1:13777/healthz >/dev/null && exit 0; \
  sleep 1; \
done; \
docker logs tag-note-e2e-remote; \
exit 1"
```

In another terminal, keep an SSH tunnel open:

```bash
ssh -N -L 13777:127.0.0.1:13777 "$E2E_HOST"
```

Run Playwright locally through the tunnel:

```bash
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e E2E_BASE_URL=http://host.docker.internal:13777 \
  -e npm_config_cache=/tmp/.npm \
  -v "$PWD":/app \
  -w /app \
  mcr.microsoft.com/playwright:v1.60.0-noble \
  npm run test:e2e
```

Stop the remote E2E app when finished:

```bash
ssh "$E2E_HOST" "docker rm -f tag-note-e2e-remote"
```

## CI Coverage

GitHub Actions currently runs:

- Dockerfile `test` stage: formatting, `go vet ./...`, `go test ./...`, and
  `govulncheck ./...`.
- Docker image build.
- Playwright E2E tests against a Dockerized app.

## Legacy Data

Older data created before multi-user authentication may belong to the legacy
placeholder user. Use `tagnote-migrate` only after creating the target account
and backing up the database.
