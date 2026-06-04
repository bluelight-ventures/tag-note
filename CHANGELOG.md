# Changelog

All notable changes to TagNote are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [1.6.3] - 2026-06-04

### Fixed

- Fixed Google sign-in on the web app. When `GOOGLE_CLIENT_ID` is a
  comma-separated list of accepted token audiences (web, iOS), the browser is
  now initialized with only the first (web) client ID; previously the whole
  list was injected, which Google rejected with `invalid_client`.

### Changed

- Added regression coverage for the web Google client-ID injection (a Go unit
  test plus a Playwright e2e check), and configured the CI/e2e server with a
  multi-audience `GOOGLE_CLIENT_ID`.

---

## [1.6.2] - 2026-06-04

### Added

- Added Sign in with Apple on iOS, backed by a new `/auth/apple` endpoint that
  verifies Apple identity tokens (JWKS signature, issuer, audience, and nonce)
  and links or creates accounts by Apple ID, then email.
- Added Sign in with Google on iOS using the official Google-branded button.
- Added a `© Bluelight Inc.` copyright notice and a `support@bluelight.ventures`
  contact across the public landing, privacy, terms, and support pages.

### Changed

- The Google login endpoint now accepts multiple client-ID audiences via a
  comma-separated `GOOGLE_CLIENT_ID`, so native iOS tokens are validated
  alongside the web client.
- Named Bluelight Inc. as the operator of TagNote in the Privacy Policy and
  Terms of Service.
- Restyled the privacy, terms, and support pages to share the landing page's
  palette and branding.
- Passed `APPLE_CLIENT_ID` through to the production container configuration.

### Fixed

- Aligned the Google sign-in button with the Apple button on the iOS auth
  screen for a consistent layout.

---

## [1.6.1] - 2026-06-03

### Added

- Added a public `/support` page for App Store Connect, including contact,
  getting-started, account deletion, and FAQ guidance.
- Added App Store screenshot capture coverage for the iOS app's notes, search,
  tags, and editor surfaces.

### Changed

- Made App Store iOS builds use the hosted `tag-note.com` instance directly,
  while keeping custom server configuration available in Debug builds.
- Updated product copy and documentation to remove self-hosted wording from the
  shipped TagNote experience.
- Hardened the iOS E2E suite for App Store screenshot generation across iPhone
  and iPad destinations.

### Fixed

- Updated the Dockerized Go build/test base image to Go 1.26.4 to pick up
  standard-library vulnerability fixes reported by `govulncheck`.
- Removed the alpha channel from the iOS marketing icon so App Store Connect
  accepts the asset.
- Added deterministic iOS coverage for compact drawer and regular-width
  persistent sidebar layouts.

---

## [1.6.0] - 2026-06-02

### Added

- Tracked upload ownership so authenticated image uploads are tied to the user
  who created them, and account deletion now removes that user's uploaded files
  (tracked uploads plus safe legacy `/uploads` references) from the configured
  upload directory.

### Changed

- Configured iOS app code signing for App Store distribution.
- Ran the frontend E2E suite in parallel across multiple workers (~6-7s vs ~18s
  serial), scoping each test's notes and tags to a globally unique suffix so the
  shared seeded account is safe under concurrency.

### Fixed

- Added a SQLite `busy_timeout` so concurrent writers wait for the single-writer
  lock instead of failing immediately with `SQLITE_BUSY`, improving reliability
  for real concurrent users as well as parallel tests.
- Passed `TAGNOTE_DOMAIN` into the Caddy container so TLS certificate issuance
  stays correct across container recreates instead of falling back to a default
  domain.
- Attached Caddy and TagNote to the monitoring network declaratively in Compose
  so the `/grafana` upstream survives container recreates without manual network
  reconnects.

---

## [1.5.0] - 2026-05-31

### Added

- Added a native iOS TagNote app with authentication, note browsing, search,
  tag filtering, editing, markdown preview, trash, tag management, settings,
  image upload support, offline cache fallback, unit tests, and UI test
  scaffolding.
- Added an iOS note reader panel and active tag-filter chips for parity with
  the web app.
- Added a protected admin Metrics section that reads `/metrics` through the
  existing admin JWT.
- Added cross-platform UX guidelines and a shorter UX principles brief for
  TagNote clients.
- Added iOS account deletion and privacy metadata for App Store readiness.

### Changed

- Made the iOS app universal and width-adaptive across iPhone, iPad,
  landscape, Split View, and Stage Manager windows.
- Aligned iOS visual treatment with the shared TagNote design system, including
  themed priority colors, save-status states, tag chips, empty states, and
  responsive note feeds.
- Updated production Caddy configuration to import co-hosted site drop-ins from
  `/etc/caddy/sites/*.caddy` without deploys overwriting those site files.

### Fixed

- Kept iOS sidebar and drawer content clear of safe areas, status areas, and
  iPad window controls.
- Restored the persistent iPad sidebar in wide windows while preserving the
  compact drawer layout for narrow windows.
- Improved iOS test documentation for deterministic screenshot verification.

---

## [1.4.1] - 2026-05-28

### Added

- Added autosave while creating and editing notes, including guest-mode support,
  saved/unsaved status feedback, and dirty-close behavior that understands
  autosaved changes.
- Added README product screenshot and the hosted app link.
- Added complete local development links for app, admin, health, status,
  metrics, proxy, and Grafana.
- Added `make start`, `make up`, and `make down` aliases for local lifecycle
  commands.
- Added E2E testing documentation for local Docker runs and remote SSH runs on
  staging or disposable hosts.

### Changed

- Removed the misleading editor cancel/clear button now that autosave can
  persist drafts in the background.
- Stabilized frontend E2E tests around the shared seeded account and EasyMDE
  readiness.
- Passed `ADMIN_EMAIL` through local Docker Compose for admin operational
  endpoint testing.
- Updated development startup output to print local TagNote links.
- Updated dependencies including Fiber, JWT, AWS SESv2, x/crypto,
  VictoriaMetrics metrics, and GitHub Actions.

### Fixed

- Prevented editor autofocus from stealing tag input focus.
- Improved read overlay content padding.
- Updated release and status scripts to verify production containers through
  Docker networking when the app port is not published on the host.

---

## [1.4.0] - 2026-05-27

### Added

- Refined open-source project documentation across README, contributing,
  testing, security, operations, and release guides.
- Added npm Dependabot coverage.

### Changed

- Updated Go and Alpine base image references for supported release lines.
- Generated public SEO metadata from deployment URL settings.
- Removed the stale `tagflow-server` command.
- Clarified uploaded image attachment privacy as link-private.
- Reduced public `/healthz` output to minimal liveness status.
- Added pinned `govulncheck` coverage to the Dockerfile `test` stage.

### Fixed

- Upgraded Fiber and golang-jwt to patched versions for reachable security
  vulnerabilities.
- Required explicit admin JWT or `OPERATIONAL_BEARER_TOKEN` access for
  `/status` and `/metrics`, including private-network callers.
- Rejected Google OAuth logins when Google reports an unverified email address.
- Added a timeout to Google token verification requests.
- Updated release verification scripts to read detailed version status from the
  protected `/status` endpoint instead of public `/healthz`.
- Prevented magic-link account creation when email delivery is not configured.

---

## [1.3.1] - 2026-03-05

### Added
- Magic link (passwordless) login
  - `POST /auth/magic-link` endpoint to request a login link via email
  - `POST /auth/verify-magic-link` endpoint to verify token and login
  - `magic_link_tokens` table for one-time login tokens (15 min expiry)
  - "Login without password" toggle on auth page
  - Auto-verify email on successful magic link login

### Fixed
- NOT NULL constraint error when creating users via magic link (use empty string instead of NULL for password_hash)

---

## [1.3.0] - 2026-03-05

### Added
- Guest Mode (Lazy Registration) for try-before-signup experience
  - localStorage-backed CRUD operations for notes, tags, and trash
  - Seed notes (welcome, tags, priority, markdown) for first-time guest users
  - "Try without an account" button on auth page
  - 5-note limit with conversion modal prompting account creation
  - Guest banner in sidebar with CTA to create account
  - Automatic migration of guest notes to server on registration/login
  - Landing page "Try it now — no sign-up" CTAs

### Changed
- Deployment now copies `docker-compose.prod.yml` and `Caddyfile` to server with rollback support
- Search & Filter section hidden in guest mode

### Fixed
- Trailing whitespace in deploy and rollback scripts

---

## [1.2.0] - 2026-03-05

### Added
- Admin dashboard with user management, audit logs, and overview statistics
- Prometheus-compatible metrics endpoint (`/metrics`) using VictoriaMetrics/metrics library
- Grafana + VictoriaMetrics monitoring stack for time-series visualization
- Audit logging middleware for all authenticated user actions
- Deployment scripts: `first_time_setup_grafana.sh`, `deploy_grafana.sh`, `status_grafana.sh`
- `ADMIN_EMAIL` env var to control admin access
- CHANGELOG.md for version history

### Changed
- Dev docker-compose now includes Grafana + VictoriaMetrics + Caddy
- Updated OPERATIONS.md with admin and monitoring documentation

---

## [0.9.0] - 2026-03-01

### Added
- Interactive landing page demo with masonry note layout
- Tech Stack section in README

### Changed
- Refined landing page accessibility and priority showcase
- Improved hero animation

---

## [0.8.0] - 2026-02-25

### Added
- 4 theme families (Everforest, Nord, Tokyo Night, Dracula) with light/dark variants
- Note width control setting
- SEO optimization for landing page
- 120x120 PNG icon with transparent corners

### Changed
- Overhauled theme system architecture

---

## [0.7.0] - 2026-02-20

### Added
- Amazon SES email integration for verification and password reset
- Privacy policy and terms of service pages

### Changed
- Improved onboarding with new slogan and cleaner landing page
- Added seed notes for new users

---

## [0.6.0] - 2026-02-15

### Added
- Release process with SSH deploy pipeline
- Server monitoring and health checks
- Build versioning with git tags

### Fixed
- Cross-platform build issues
- Production config cleanup for first deployment

---

## [0.5.0] - 2026-02-10

### Added
- Comprehensive import/export with tags, trash, and settings
- Timestamp preservation on import
- CLAUDE.md project guide

---

## [0.4.0] - 2026-02-05

### Added
- Google OAuth authentication
- Email verification flow
- Password reset functionality
- Redesigned auth page with tabbed login/register interface

---

## [0.3.0] - 2026-01-30

### Added
- Production deployment infrastructure for example.com
- New tag logo and branding
- Improved PWA install experience
- Import notes feature with duplicate preview

---

## [0.2.0] - 2026-01-25

### Added
- Tag management (rename, delete, priority)
- Trash and soft-delete functionality
- User settings persistence
- EasyMDE markdown editor integration

---

## [0.1.0] - 2026-01-20

### Added
- Initial release
- Note CRUD with tag extraction
- JWT authentication with bcrypt passwords
- SQLite database with WAL mode
- Vanilla JS SPA frontend
- Docker deployment support
- Basic PWA functionality

---

[Unreleased]: https://github.com/runminglu/tag-note/compare/v1.6.3...HEAD
[1.6.3]: https://github.com/runminglu/tag-note/compare/v1.6.2...v1.6.3
[1.6.2]: https://github.com/runminglu/tag-note/compare/v1.6.1...v1.6.2
[1.6.1]: https://github.com/runminglu/tag-note/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/runminglu/tag-note/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/runminglu/tag-note/compare/v1.4.1...v1.5.0
[1.4.1]: https://github.com/runminglu/tag-note/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/runminglu/tag-note/compare/v1.3.1...v1.4.0
[1.3.1]: https://github.com/runminglu/tag-note/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/runminglu/tag-note/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/runminglu/tag-note/compare/v0.9.0...v1.2.0
[0.9.0]: https://github.com/runminglu/tag-note/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/runminglu/tag-note/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/runminglu/tag-note/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/runminglu/tag-note/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/runminglu/tag-note/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/runminglu/tag-note/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/runminglu/tag-note/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/runminglu/tag-note/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/runminglu/tag-note/releases/tag/v0.1.0
