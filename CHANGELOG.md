# Changelog

All notable changes to TagNote are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
