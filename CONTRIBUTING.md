# Contributing To TagNote

Thank you for helping improve TagNote. This project aims to stay small,
self-hostable, and easy to operate.

## Ways To Contribute

- Report reproducible bugs.
- Improve documentation.
- Add focused tests.
- Fix issues in the web UI, API, CLI tools, deployment scripts, or monitoring.
- Propose features that fit the tag-first note-taking model.

## Before You Start

1. Check existing issues and pull requests.
2. Open an issue for large behavior changes before investing significant time.
3. Keep pull requests focused on one concern.
4. Avoid committing local data, `.env` files, SQLite databases, uploads, logs, or test artifacts.

## Development Setup

```bash
cp .env.example .env
docker compose build
docker compose up -d
```

The app runs at `http://localhost:3777/app`.

To enable the built-in test account:

```bash
TAGNOTE_TEST_MODE=1 docker compose build
TAGNOTE_TEST_MODE=1 docker compose up -d
```

Credentials are `test@test.com` / `testpass123`.

## Testing

Run backend tests through Docker:

```bash
docker build --target test .
```

This Docker target runs formatting checks, `go vet`, Go tests, and
`govulncheck` with the pinned tool version from the Dockerfile.

Run browser tests through Docker as described in [TESTING.md](TESTING.md).

Do not rely on a host-specific local environment for pull request validation.
The GitHub Actions workflow is the source of truth for CI.

## Pull Request Checklist

- The change is scoped and explained clearly.
- User-facing behavior has tests where practical.
- Documentation is updated when commands, configuration, API routes, or workflows change.
- Docker-based tests relevant to the change have been run.
- The pull request description includes any known limitations or follow-up work.

## Code Style

- Follow existing package boundaries: handler, service, repository, model, and middleware.
- Keep frontend changes in `web/` consistent with the existing vanilla JavaScript structure.
- Prefer simple data flows and explicit error handling over broad abstractions.
- Preserve backwards-compatible data migrations whenever possible.

## Commit Messages

Use short, imperative commit messages when possible:

```text
Add trash restore endpoint
Fix tag autocomplete ordering
Document staging deploy workflow
```

## Code Of Conduct

All contributors are expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
