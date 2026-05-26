# TagNote

A fast note-taking system that organizes your thinking with tags instead of folders.

## What is TagNote?

Every note you write gets one or more tags. Filter by any combination of tags and a live *stream* of matching notes appears instantly — no hierarchies to maintain, no folders to shuffle things between, no decisions about where something "belongs." If a note is relevant, it flows into view.

### Why TagNote?

Folders force you to choose *one place* for every idea. But ideas don't live in one place — they sit at the intersection of projects, topics, and contexts. TagNote embraces that reality. Tag a note `#backend` and `#performance` and `#q3-goals`, and it surfaces everywhere it's relevant. Combine tags to zoom in; remove them to zoom out. Your notes become a living, queryable knowledge base that reshapes itself around whatever you're focused on right now.

### How it works

- **Write** — Capture Markdown notes with a rich editor, inline images, and tag autocomplete.
- **Tag** — Assign one or more tags per note. New tags are flagged for review so your taxonomy stays clean.
- **Stream** — Filter by any intersection of tags to see exactly the notes you need. Add full-text search to narrow further.
- **Prioritize** — Set Importance and Urgency on each tag. Notes are color-coded by Eisenhower quadrant so you always know what matters most.

### Lightweight by design

TagNote ships as a **single binary** with the web UI embedded — no Node.js, no build step, no external services. It uses SQLite for storage, so your data lives in one portable file. Deploy with Docker in seconds or run the bare binary directly. Get started in under a minute.

Multi-user support, JWT authentication, a full REST API, CLI tools, five color themes, PWA installability — everything you need, nothing you don't.

**Your notes. Your tags. Your flow.**

## Quick Start

```bash
# Copy example env and customize (optional for dev)
cp .env.example .env

docker compose up --build
```

| URL | Description |
|-----|-------------|
| `http://localhost:3777` | Landing page |
| `http://localhost:3777/app` | App (login / register / notes) |

Open the landing page to learn about TagNote, or go straight to `/app` to register and start writing.

### Development / Test Mode

To pre-seed a test account (`test@test.com` / `testpass123`), opt in explicitly:

```bash
TAGNOTE_TEST_MODE=1 docker compose up --build
```

## Features

- **Tag-based organization** — Every note has one or more tags. Filter by any intersection of tags (AND logic) to see exactly the notes you need.
- **Full-text search** — FTS5-powered search across all note content, combinable with tag filters.
- **Rich Markdown editing** — EasyMDE editor with toolbar, side-by-side preview, and image upload (paste, drag-and-drop, or button).
- **Image uploads** — JPEG, PNG, GIF, and WebP up to 5 MB. Images are auto-compressed client-side before upload.
- **Masonry card layout** — Notes render as cards in a responsive column grid with collapsible long content.
- **Tag management** — Approve, rename, merge, or delete tags. New tags start as "unreviewed" with a badge count.
- **Priority system** — Each tag has Importance (0–100) and Urgency (0–100) sliders. Notes are color-coded by the Eisenhower quadrant of their highest-priority tag.
- **Color themes** — Light, Dark, Nord, Solarized, and Rose Pine. Respects system preference on first visit.
- **Multi-user** — JWT-based authentication. Each user has isolated notes and tags.
- **PWA** — Installable as a standalone app from `/app`. Service worker and manifest are scoped to `/app` — the landing page is not part of the PWA.
- **CLI tools** — Full-featured command-line clients that talk to the same API as the web UI.

## Web UI

The server serves two distinct pages:

| Route | Content |
|-------|---------|
| `/` | Landing page — product overview, features, and CTA to open the app |
| `/app` | Single-page application — login, notes, tags, editor |

Static assets (`/style.css`, `/app.js`, icons) are shared and served from the root. The PWA (manifest + service worker) is scoped to `/app` only — the landing page has no service worker and no install prompt.

The SPA at `/app` provides:

- **Sidebar** — Tag cloud, tag/text filters, "New note" button, navigation between Notes and Tags views.
- **Note cards** — Rendered Markdown with inline edit, delete, and full-screen read actions. Clickable tag pills to filter.
- **Focus overlay** — Full-screen editor for creating or editing notes with tag chip input and autocomplete.
- **Tag management view** — Table of all tags with status, note count, priority sliders, and bulk actions (approve, rename, delete).

## API

All endpoints (except auth) require a `Bearer` token in the `Authorization` header.

### Authentication

| Method | Endpoint                          | Description                              |
|--------|-----------------------------------|------------------------------------------|
| `POST` | `/api/v1/auth/register`           | Create account (sends verification email)|
| `POST` | `/api/v1/auth/login`              | Login, returns JWT                       |
| `POST` | `/api/v1/auth/logout`             | Logout (client-side token drop)          |
| `GET`  | `/api/v1/auth/me`                 | Current user info                        |
| `POST` | `/api/v1/auth/google`             | Google OAuth login                       |
| `POST` | `/api/v1/auth/verify-email`       | Verify email with token                  |
| `POST` | `/api/v1/auth/resend-verification`| Resend verification email                |
| `POST` | `/api/v1/auth/forgot-password`    | Request password reset email             |
| `POST` | `/api/v1/auth/reset-password`     | Reset password with token                |

### Notes

| Method   | Endpoint                     | Description                          |
|----------|------------------------------|--------------------------------------|
| `POST`   | `/api/v1/notes`              | Create a note                        |
| `GET`    | `/api/v1/notes?tag=X&tag=Y`  | List notes (JSON). AND logic on tags |
| `GET`    | `/api/v1/notes?q=search`     | Full-text search (combinable with tags) |
| `GET`    | `/api/v1/notes/stream?tag=X` | Markdown stream                      |
| `GET`    | `/api/v1/notes/:id`          | Get a single note                    |
| `PUT`    | `/api/v1/notes/:id`          | Update content and/or tags           |
| `DELETE` | `/api/v1/notes/:id`          | Delete a note                        |

### Tags

| Method   | Endpoint                        | Description                          |
|----------|---------------------------------|--------------------------------------|
| `GET`    | `/api/v1/tags`                  | List tag names (sorted by recency)   |
| `GET`    | `/api/v1/tags/detailed`         | List tags with status, count, priority |
| `GET`    | `/api/v1/tags/autocomplete?q=X` | Prefix-match autocomplete            |
| `PUT`    | `/api/v1/tags/:name/approve`    | Mark tag as approved                 |
| `PUT`    | `/api/v1/tags/:name/rename`     | Rename or merge a tag                |
| `PUT`    | `/api/v1/tags/:name/priority`   | Set importance/urgency (0–100)       |
| `DELETE` | `/api/v1/tags/:name`            | Delete a tag (notes are kept)        |

### Images

| Method | Endpoint          | Description                     |
|--------|-------------------|---------------------------------|
| `POST` | `/api/v1/images`  | Upload image (multipart, max 5 MB) |

### Example

```bash
# Register
curl -X POST http://localhost:3777/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email": "me@example.com", "password": "mypassword", "display_name": "Me"}'

# Login
TOKEN=$(curl -s -X POST http://localhost:3777/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "me@example.com", "password": "mypassword"}' | jq -r .token)

# Create a note
curl -X POST http://localhost:3777/api/v1/notes \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content": "Hello world", "tags": ["demo"]}'

# Search notes
curl "http://localhost:3777/api/v1/notes?q=hello" \
  -H "Authorization: Bearer $TOKEN"
```

## CLI Usage

CLI tools are thin HTTP clients that talk to the running server. They require authentication.

### Setup

```bash
# Login and get a token
docker compose exec tagnote tagnote-login
# Follow the prompts, then export the token:
export TAGNOTE_TOKEN=<token>

# Point at a remote server (default: http://localhost:3000)
export TAGNOTE_URL=http://your-server:3777
```

### Commands

```bash
# Add a note with tags
tagnote-add -t project -t ideas "Build a tag-based notes system"

# Read a markdown stream filtered by tags
tagnote-read -t project

# Full-text search within a tag
tagnote-read -t project -s "stream"

# View metadata log (ID, timestamp, tags, snippet)
tagnote-logs -t project

# Search across all notes
tagnote-logs -s "keyword"

# Delete a note by short ID
tagnote-delete <id>

# Manage tags
tagnote-tags                        # List all tags with status and note count
tagnote-tags approve <name>         # Approve an unreviewed tag
tagnote-tags rename <old> <new>     # Rename or merge a tag
tagnote-tags delete <name>          # Delete a tag
```

## Tech Stack

### Backend

- **Go 1.22** — statically compiled, single binary deployment
- **Fiber v2** — high-performance web framework built on fasthttp
- **SQLite** — embedded database via pure-Go driver (`modernc.org/sqlite`, no CGO)
- **FTS5** — full-text search engine (SQLite extension)
- **JWT** — stateless authentication (`golang-jwt/jwt`)
- **bcrypt** — password hashing (`golang.org/x/crypto`)
- **ULID** — lexicographically sortable unique IDs (`oklog/ulid`)
- **AWS SES v2** — transactional email (verification, password reset)

### Frontend

- **Vanilla JavaScript** — zero frameworks, no build step, no npm
- **EasyMDE** — Markdown editor with toolbar and live preview
- **PWA** — service worker, web manifest, offline support, installable
- **CSS** — single stylesheet, 8 themes across 4 families (Everforest, Solarized, Gruvbox, Nord — each light/dark)

### Infrastructure

- **Docker** — multi-stage build (Alpine), runs as non-root user
- **`go:embed`** — entire frontend is embedded in the Go binary
- **GitHub Actions** — CI (build + integration tests) and release (cross-compile + GHCR push)
- **Cross-platform** — `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`

## Architecture

```
cmd/
  tagnote-server/    # Fiber HTTP server + embedded SPA
  tagnote-add/       # CLI: add note
  tagnote-read/      # CLI: read markdown stream
  tagnote-logs/      # CLI: metadata log view
  tagnote-delete/    # CLI: delete note
  tagnote-tags/      # CLI: tag management (list, approve, rename, delete)
  tagnote-login/     # CLI: interactive login, prints JWT
  tagnote-migrate/   # CLI: migrate legacy (pre-auth) data to a user account
  tagnote-diagnose/  # CLI: database integrity checks
internal/
  model/         # Domain types (SubNote, User, TagInfo)
  repo/          # SQLite repository (migrations, queries, FTS5)
  service/       # Business logic (ULID generation, tag normalization, auth)
  handler/       # Fiber route handlers (notes, tags, auth, images)
  middleware/    # JWT authentication middleware
  apiclient/     # Shared HTTP client for CLI tools
web/             # Embedded SPA (HTML, JS, CSS, PWA manifest)
```

**Stack:** Go 1.22, Fiber v2, SQLite (pure-Go via `modernc.org/sqlite`), JWT (`golang-jwt`), bcrypt, ULIDs.

**Data model:** `users` → `subnotes` ↔ `subnote_tags` ↔ `tags` (many-to-many, scoped per user). Full-text search via `subnotes_fts` (FTS5). Tag intersection queries use `HAVING COUNT(DISTINCT t.id) = N`.

**Routing:** `GET /` → `landing.html`, `GET /app*` → `index.html` (SPA catch-all), static assets served from embedded `web/` directory. PWA manifest and service worker scoped to `/app`.

**Frontend:** Vanilla JS SPA embedded in the Go binary. EasyMDE for Markdown editing. No build tooling.

## Deployment

### Development

```bash
docker compose up --build
```

Maps port 3777 → container port 3000. Data persists in a Docker volume (`tagnote-data`).

### Production

```bash
# Set required env vars
export TAGNOTE_DOMAIN=notes.example.com
export JWT_SECRET=$(openssl rand -hex 32)

docker compose -f docker-compose.yml -f docker-compose.prod.yml up --build -d
```

The production override exposes ports 80/443 and requires `TAGNOTE_DOMAIN` and `JWT_SECRET`.

### Releasing

Tag a version to trigger the release pipeline:

```bash
git tag v1.0.0
git push origin v1.0.0
```

This builds cross-platform binaries, pushes a Docker image to GHCR, and creates a GitHub Release with downloadable tarballs.

## CI/CD

GitHub Actions pipelines live in `.github/workflows/`.

### CI (`ci.yml`)

Runs on every push and pull request to `main`. Two jobs:

| Job | What it does |
|-----|-------------|
| **build** | Sets up Go 1.22, generates OG image (SVG→PNG), downloads deps, runs `go vet`, builds all `tagnote-*` binaries |
| **docker** | Builds Docker image, starts container with test user, runs integration tests against live server |

Integration tests cover:

- Landing page (`/`) returns 200
- App (`/app`) returns 200
- All static assets (CSS, JS, icons, OG image, manifest, service worker) return 200
- Auth flow: login → create note → list notes → verify 401 without token

### Release (`release.yml`)

Runs on version tags (`v*`). Performs:

| Step | Output |
|------|--------|
| Cross-compile | `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` tarballs |
| Docker push | Image pushed to `ghcr.io` with semver tags (`v1.0.0`, `v1.0`, `v1`, `latest`) |
| GitHub Release | Auto-generated release notes with binary tarballs attached |

### Running CI locally

Simulate the CI build job without GitHub Actions:

```bash
# Generate OG image (requires librsvg)
brew install librsvg          # macOS
rsvg-convert web/og-image.svg -o web/og-image.png

# Vet and build
go vet ./...
for dir in cmd/tagnote-*/; do
  CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(basename $dir) ./$dir
done

# Or just use Docker (no local Go/librsvg needed)
docker compose up --build
```

### Environment Variables

| Variable           | Default              | Description                              |
|--------------------|----------------------|------------------------------------------|
| `JWT_SECRET`       | `tagnote-dev-secret` | HMAC key for signing JWTs. **Change in production.** |
| `TAGNOTE_TEST_MODE`| `0`                  | Set to `1` to auto-create a test account |
| `TAGNOTE_URL`      | `http://localhost:3000` | Server URL for CLI tools              |
| `TAGNOTE_TOKEN`    | (none)               | JWT token for CLI authentication         |
| `TAGNOTE_DOMAIN`   | (none)               | Domain name (production deployment)      |

#### Email Configuration

Email is required for email verification and password reset features. If not configured, accounts are auto-verified on registration.

**Priority order:** Amazon SES → SMTP → sendmail. The first configured option is used.

##### Amazon SES (Recommended for Production)

| Variable             | Default      | Description                              |
|----------------------|--------------|------------------------------------------|
| `AWS_SES_ACCESS_KEY` | (none)       | AWS access key ID for SES                |
| `AWS_SES_SECRET_KEY` | (none)       | AWS secret access key for SES            |
| `AWS_SES_REGION`     | `us-east-1`  | AWS region for SES (e.g., `us-east-1`, `eu-west-1`) |

To use Amazon SES:
1. Create an IAM user with `AmazonSESFullAccess` permission (or a more restrictive custom policy)
2. Generate access keys for the IAM user
3. Verify your sender email address or domain in the SES console
4. If in sandbox mode, also verify recipient addresses

##### SMTP

| Variable        | Default               | Description                              |
|-----------------|-----------------------|------------------------------------------|
| `SMTP_HOST`     | (none)                | SMTP server hostname                     |
| `SMTP_PORT`     | `587`                 | SMTP server port                         |
| `SMTP_USER`     | (none)                | SMTP authentication username             |
| `SMTP_PASSWORD` | (none)                | SMTP authentication password             |

##### Sendmail

| Variable        | Default | Description                              |
|-----------------|---------|------------------------------------------|
| `USE_SENDMAIL`  | `0`     | Set to `1` to use system sendmail binary |

##### Common Settings

| Variable        | Default               | Description                              |
|-----------------|-----------------------|------------------------------------------|
| `EMAIL_FROM`    | `noreply@example.com` | From address for outgoing emails         |
| `BASE_URL`      | `http://localhost:3000` | Base URL for links in emails (e.g., `https://notes.example.com`) |

**Email delivery behavior:**
- **Amazon SES configured** → Emails sent via AWS SES API (recommended for production)
- **SMTP configured** → Emails sent via SMTP server
- **USE_SENDMAIL=1** → Uses system `sendmail` command (for servers with working MTA)
- **None configured** → Email features disabled, accounts are auto-verified on registration

#### Google OAuth (Optional)

| Variable            | Default | Description                              |
|---------------------|---------|------------------------------------------|
| `GOOGLE_CLIENT_ID`  | (none)  | Google OAuth Client ID for "Sign in with Google". If not set, the Google button is hidden. |

To enable Google Sign-In:
1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Go to **APIs & Services** → **Credentials**
4. Create an **OAuth 2.0 Client ID** (Web application)
5. Add your domain to **Authorized JavaScript origins** (e.g., `https://notes.example.com`)
6. Copy the Client ID and set `GOOGLE_CLIENT_ID`

### Server Flags

```
tagnote-server [-addr :3000] [-db data/tagnote.db] [-uploads data/uploads]
```

| Flag       | Default           | Description                  |
|------------|-------------------|------------------------------|
| `-addr`    | `:3000`           | Listen address               |
| `-db`      | `data/tagnote.db` | Path to SQLite database file |
| `-uploads` | `data/uploads`    | Path to image upload directory |
