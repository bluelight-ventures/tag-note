# TagNote MCP Server Design

This document designs a Model Context Protocol server that lets an LLM read and
act on a user's TagNote data through a native Streamable HTTP MCP endpoint.

## Goals

- Let an LLM search, read, create, update, tag, pin, and organize notes.
- Reuse TagNote's current service behavior, validation, SQLite migrations, and
  deployment model.
- Use the same long-running HTTP MCP interface locally and in production.
- Use native MCP OAuth 2.1 authorization instead of TagNote JWT setup.
- Keep destructive actions opt-in and leave permanent deletion out of the
  default tool surface.

## Non-Goals

- Do not expose admin APIs, account deletion, operational status, metrics, or
  raw database access.
- Do not build an LLM into TagNote. The MCP server only exposes TagNote context
  and actions to MCP clients.
- Do not add UI changes to the web app or iOS app for the first release.
- Do not keep a second stdio transport or TagNote-JWT MCP mode.

## Current TagNote Integration Points

The implementation should start from these existing paths:

| Path | Role |
| --- | --- |
| `/Users/runming/workspace/tag_note/internal/model/model.go` | Shared request/response and domain structs. |
| `/Users/runming/workspace/tag_note/internal/service/service.go` | Business behavior behind notes, tags, trash, import/export, and settings. |
| `/Users/runming/workspace/tag_note/internal/repo/migrate.go` | SQLite schema for notes, users, and MCP OAuth state. |
| `/Users/runming/workspace/tag_note/internal/mcpoauth/` | OAuth metadata, dynamic registration, authorization, token, and verifier behavior. |
| `/Users/runming/workspace/tag_note/Dockerfile` | Canonical build/test path and final image binary list. |

The MCP server is repository-backed through `service.Service`, not an HTTP API
client. That keeps local and production identical and avoids minting broad
TagNote JWTs for MCP hosts.

## MCP Shape

Use the official Go SDK:

```text
github.com/modelcontextprotocol/go-sdk/mcp
```

Pin an exact SDK version with Dockerized Go tooling. As of 2026-06-06, the
official Go SDK is listed as Tier 1 by MCP documentation, and GitHub lists
`v1.6.1` as the latest release. If a newer stable patched release exists when
implementation starts, intentionally choose and pin that exact version.

### Binary

Add:

```text
/Users/runming/workspace/tag_note/cmd/tagnote-mcp/main.go
/Users/runming/workspace/tag_note/internal/mcpserver/
```

Default command:

```bash
tagnote-mcp
```

Default transport:

```text
Streamable HTTP
```

Production endpoint:

```text
https://mcp.tag-note.com/mcp
```

Local endpoint:

```text
http://localhost:3779/mcp
```

Required runtime configuration:

| Variable | Purpose |
| --- | --- |
| `TAGNOTE_MCP_PUBLIC_URL` | Public MCP origin, defaulting to `http://localhost:3779` locally and `https://mcp.tag-note.com` in production. |
| `TAGNOTE_DB` | SQLite database path. |
| `TAGNOTE_UPLOADS` | Upload directory used by existing auth/account cleanup behavior. |
| `JWT_SECRET` | Existing TagNote secret, reused only to sign short-lived MCP browser-login session cookies. |

## Native MCP Authorization

TagNote should implement the MCP Authorization Specification for HTTP-based MCP
transport. In this model:

- `https://mcp.tag-note.com/mcp` is the MCP protected resource.
- `https://mcp.tag-note.com` is the OAuth authorization server for MCP clients.
- The MCP server is an OAuth 2.1 resource server.
- The MCP client is an OAuth 2.1 public client using PKCE.
- The user authenticates through existing TagNote login methods.
- The issued access token is scoped and audience-bound for
  `https://mcp.tag-note.com/mcp`.

### Discovery Endpoints

MCP clients must be able to discover authorization without prior TagNote-specific
configuration.

Protected resource metadata:

```text
GET https://mcp.tag-note.com/.well-known/oauth-protected-resource/mcp
```

Response:

```json
{
  "resource": "https://mcp.tag-note.com/mcp",
  "authorization_servers": ["https://mcp.tag-note.com"],
  "bearer_methods_supported": ["header"],
  "scopes_supported": ["mcp:read", "mcp:write", "mcp:delete"]
}
```

Authorization server metadata:

```text
GET https://mcp.tag-note.com/.well-known/oauth-authorization-server
```

Response:

```json
{
  "issuer": "https://mcp.tag-note.com",
  "authorization_endpoint": "https://mcp.tag-note.com/oauth/authorize",
  "token_endpoint": "https://mcp.tag-note.com/oauth/token",
  "registration_endpoint": "https://mcp.tag-note.com/oauth/register",
  "jwks_uri": "https://mcp.tag-note.com/.well-known/jwks.json",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "code_challenge_methods_supported": ["S256"],
  "token_endpoint_auth_methods_supported": ["none"],
  "scopes_supported": ["mcp:read", "mcp:write", "mcp:delete"]
}
```

When an unauthenticated request reaches `/mcp`, return:

```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer resource_metadata="https://mcp.tag-note.com/.well-known/oauth-protected-resource/mcp"
```

### Dynamic Client Registration

Support OAuth Dynamic Client Registration for MCP clients:

```text
POST https://mcp.tag-note.com/oauth/register
```

Accepted registration metadata:

| Field | Requirement |
| --- | --- |
| `client_name` | Optional display name for the user's consent screen. |
| `redirect_uris` | Required. Must be HTTPS or loopback localhost. |
| `grant_types` | Must include `authorization_code`; may include `refresh_token`. |
| `response_types` | Must be `code`. |
| `token_endpoint_auth_method` | Must be `none` for public MCP clients. |
| `scope` | Optional subset of supported MCP scopes. |

Store registered clients in SQLite:

```text
oauth_clients(
  client_id TEXT PRIMARY KEY,
  client_name TEXT,
  redirect_uris TEXT NOT NULL,
  scopes TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  last_used_at TIMESTAMP
)
```

Client secrets are not issued for public MCP clients in the first release.

### Authorization Code Flow With PKCE

Authorization endpoint:

```text
GET https://mcp.tag-note.com/oauth/authorize
```

Required query parameters:

| Parameter | Requirement |
| --- | --- |
| `response_type` | Must be `code`. |
| `client_id` | Must identify a registered MCP client. |
| `redirect_uri` | Must exactly match one registered URI. |
| `code_challenge` | Required. |
| `code_challenge_method` | Must be `S256`. |
| `state` | Required by clients; echo unchanged. |
| `resource` | Must be `https://mcp.tag-note.com/mcp`. |
| `scope` | Optional subset of `mcp:read mcp:write mcp:delete`. |

Flow:

1. If the user does not have a TagNote session, show the existing TagNote login
   screen and preserve the OAuth request.
2. After login, show a consent screen with client name, redirect URI, requested
   scopes, and target resource.
3. On approval, create a short-lived authorization code bound to:
   - user ID
   - client ID
   - redirect URI
   - code challenge
   - resource
   - scopes
4. Redirect to `redirect_uri?code=...&state=...`.

Authorization codes must be single-use and expire within 5 minutes.

### Token Endpoint

Token endpoint:

```text
POST https://mcp.tag-note.com/oauth/token
```

Supported grants:

| Grant | Purpose |
| --- | --- |
| `authorization_code` | Exchange a code plus `code_verifier` for tokens. |
| `refresh_token` | Rotate refresh token and issue a new access token. |

Access tokens:

- Are opaque bearer tokens stored hashed at rest.
- Bind to TagNote user ID, OAuth client ID, scope, and expiry in SQLite.
- Expire quickly, currently one hour.

Refresh tokens:

- Store only hashed refresh tokens.
- Rotate on every use.
- Bind to user ID, client ID, and resource.
- Expire after a bounded lifetime, for example 30 days.

### MCP Request Authorization

Every HTTP request to `/mcp` must include:

```http
Authorization: Bearer <access-token>
```

The MCP service validates:

- opaque token hash lookup
- token expiry
- per-tool OAuth scope
- session user consistency through the MCP SDK's bearer middleware
- expiration and not-before timestamps
- issuer
- audience/resource binding
- scope required for the called tool
- user still exists
- registered client still exists

Scope mapping:

| Scope | Enables |
| --- | --- |
| `mcp:read` | Search, get note, render stream, list tags, read resources, prompts. |
| `mcp:write` | Create, update, pin, restore, approve, rename, priority updates. |
| `mcp:delete` | Soft-delete note and delete tag. Permanent delete remains unavailable. |

The MCP service must not pass the MCP access token through to the TagNote HTTP
API. It should validate the token, derive the TagNote user ID, and call shared
service/repository code directly or mint an internal-only short-lived service
credential that cannot be used outside the deployment.

### OAuth Persistence

Add repository-backed storage:

```text
oauth_clients
oauth_authorization_codes
oauth_refresh_tokens
oauth_consents
oauth_signing_keys
```

`oauth_consents` should let users revoke MCP clients later from account
settings.

### Security Requirements

- HTTPS is mandatory for production authorization and token endpoints.
- Redirect URI matching must be exact.
- Loopback redirect URIs are allowed for local desktop MCP clients.
- Access tokens must not be accepted from URI query parameters.
- Authorization codes and refresh tokens must be stored hashed.
- Consent should be explicit for write/delete scopes.
- Delete tools remain disabled unless both the deployment and token scope allow
  `mcp:delete`.
- Audit logs should record MCP client ID, user ID, tool name, status, and
  source IP without storing token material.

### Backward Compatibility

The stdio and TagNote-JWT MCP modes are removed. Local development uses
`http://localhost:3779/mcp`, which has the same interface as production.

Optional environment:

| Variable | Default | Purpose |
| --- | --- | --- |
| `TAGNOTE_MCP_READ_ONLY` | `0` | When `1`, only read tools/resources are registered. |
| `TAGNOTE_MCP_ALLOW_DELETE` | `0` | Enables soft-delete and tag-delete tools. |
| `TAGNOTE_MCP_MAX_NOTES` | `50` | Maximum notes returned by search tools. |
| `TAGNOTE_MCP_MAX_CONTENT_BYTES` | `200000` | Maximum total content bytes returned in one tool/resource response. |

## Tool Surface

Tool names use a `tagnote_` prefix so hosts with multiple MCP servers can route
calls clearly.

### Read Tools

| Tool | Input | Output | API Backing |
| --- | --- | --- | --- |
| `tagnote_search_notes` | `tags[]`, `query`, `limit`, `offset`, `sort`, `include_content` | Matching notes, capped by server policy. | `GET /api/v1/notes` |
| `tagnote_get_note` | `id` | One note by full ID or unambiguous short ID. | `GET /api/v1/notes/:id` |
| `tagnote_render_stream` | `tags[]`, `query` | Markdown stream of matching notes, capped. | `GET /api/v1/notes/stream` |
| `tagnote_list_tags` | `detailed`, `limit` | Tag names or detailed tag metadata. | `GET /api/v1/tags`, `GET /api/v1/tags/detailed` |
| `tagnote_autocomplete_tags` | `prefix`, `limit` | Matching tag names. | `GET /api/v1/tags/autocomplete` |
| `tagnote_list_trash` | `limit`, `offset` | Trashed notes, capped locally if API does not paginate yet. | `GET /api/v1/notes/trash` |
| `tagnote_get_settings` | none | User settings. | `GET /api/v1/settings` |

### Write Tools

| Tool | Input | Output | API Backing | Safety |
| --- | --- | --- | --- | --- |
| `tagnote_create_note` | `content`, `tags[]`, `pinned` | Created note ID and note snapshot. | `POST /api/v1/notes`, optional pin toggle | Enabled unless read-only. |
| `tagnote_update_note` | `id`, optional `content`, optional `tags[]` | Updated note snapshot. | `PUT /api/v1/notes/:id` | Enabled unless read-only. |
| `tagnote_set_note_pinned` | `id`, `pinned` | Updated pin state. | Read note, call `PUT /api/v1/notes/:id/pin` only if needed. | Idempotent at MCP layer. |
| `tagnote_restore_note` | `id` | Restored note ID. | `PUT /api/v1/notes/:id/restore` | Enabled unless read-only. |
| `tagnote_approve_tag` | `name` | Approved tag name. | `PUT /api/v1/tags/:name/approve` | Enabled unless read-only. |
| `tagnote_rename_tag` | `old_name`, `new_name` | Rename/merge result. | `PUT /api/v1/tags/:name/rename` | Enabled unless read-only. |
| `tagnote_update_tag_priority` | `name`, `importance`, `urgency` | Updated priority. | `PUT /api/v1/tags/:name/priority` | Enabled unless read-only. |
| `tagnote_delete_note` | `id` | Soft-deleted note ID. | `DELETE /api/v1/notes/:id` | Register only when `TAGNOTE_MCP_ALLOW_DELETE=1`. |
| `tagnote_delete_tag` | `name` | Deleted tag name. | `DELETE /api/v1/tags/:name` | Register only when `TAGNOTE_MCP_ALLOW_DELETE=1`. |

Do not expose `DELETE /api/v1/notes/:id/permanent` in the first release. If it
is ever added, it should require a separate `TAGNOTE_MCP_ALLOW_PERMANENT_DELETE`
flag and should be marked destructive in tool metadata.

## Resource Surface

Resources are read-only context. Use a custom URI scheme:

| Resource | MIME Type | Purpose |
| --- | --- | --- |
| `tagnote://tags` | `application/json` | Detailed tag index with counts and priorities. |
| `tagnote://settings` | `application/json` | Current user settings. |
| `tagnote://trash` | `application/json` | Trashed notes, capped. |
| `tagnote://export/summary` | `application/json` | Export metadata and counts, not full note content. |

Resource templates:

| Template | MIME Type | Purpose |
| --- | --- | --- |
| `tagnote://notes/{id}` | `application/json` | Fetch one note by full or short ID. |
| `tagnote://notes/{id}.md` | `text/markdown` | Fetch one note content as Markdown. |
| `tagnote://search{?tag,q,limit,sort}` | `application/json` | Search note metadata and optional snippets. |
| `tagnote://stream{?tag,q}` | `text/markdown` | Read a capped Markdown stream. |

The resource and tool implementations should share the same service methods and
response capping code.

## Prompt Surface

Prompts are optional, but useful because TagNote has strong product concepts.
Add these after the core tools:

| Prompt | Arguments | Purpose |
| --- | --- | --- |
| `capture_note` | `source_text`, `suggested_tags[]` | Convert current conversation or selected text into a concise TagNote note. |
| `summarize_tag` | `tag`, optional `query` | Summarize a filtered note stream and cite note IDs. |
| `organize_tags` | optional `query` | Review unapproved or low-signal tags and propose renames/merges before using write tools. |
| `weekly_review` | `tags[]`, optional `query` | Build a review from recent or priority-focused notes. |

Prompts should instruct the model to search/read first, cite note IDs in any
claim based on note content, and ask before broad write operations.

## Service Work

MCP handlers use `/Users/runming/workspace/tag_note/internal/service/service.go`
directly with the user ID from OAuth token metadata. This keeps note and tag
behavior shared with the existing web API while avoiding a second HTTP hop and
avoiding TagNote JWTs in MCP hosts.

## Server Package Design

Add `/Users/runming/workspace/tag_note/internal/mcpserver/` with:

```text
config.go      environment and flag parsing
server.go      MCP server construction and tool/resource registration
tools.go       tool handlers
resources.go   resource handlers and URI parsing
prompts.go     prompt definitions
caps.go        result size caps and note redaction helpers
auth.go        token metadata and scope helpers
```

Core types:

```go
type Config struct {
    Addr            string
    DBPath          string
    UploadsDir      string
    PublicURL       string
    ResourcePath    string
    ReadOnly        bool
    AllowDelete     bool
    MaxNotes        int
    MaxContentBytes int
}

type Server struct {
    cfg     Config
    service *service.Service
}
```

All handlers should accept `context.Context`, propagate it to HTTP requests,
and return structured JSON output where possible. Text content is useful for
Markdown streams, but JSON output makes tool results easier for hosts to reason
about.

## Security And Privacy

- The MCP server acts with the exact permissions of the bearer token attached
  to each HTTP MCP request.
- Bearer tokens must not be logged.
- Return note contents only because the user explicitly connected the MCP
  server to an LLM host. Documentation should state this plainly.
- Keep result caps on by default to avoid accidentally sending an entire
  notebook to the model.
- Do not expose raw upload filesystem paths. Preserve existing `/uploads/...`
  links only as note content.
- Audit logging for MCP tool calls should be added with user ID, client ID,
  tool/resource name, status, and source IP without token material.
- Disable destructive tools unless explicitly enabled.
- Prefer idempotent tools. For pinning, implement `set_note_pinned` by reading
  current state before toggling.

## Deployment

### Local And Production HTTP

Run the MCP server as a long-running service in Docker Compose. Local and
production should use the same HTTP MCP transport:

```text
local:      http://localhost:3779/mcp
production: https://mcp.tag-note.com/mcp
```

The preferred service design is:

- `tagnote-mcp -addr :3001`
- Require `Authorization: Bearer <MCP OAuth access token>` on MCP HTTP
  requests.
- Validate opaque OAuth tokens through `/Users/runming/workspace/tag_note/internal/mcpoauth/`.
- Keep per-request user scoping from token metadata, not from tool arguments.
- Route production through Caddy at `https://mcp.tag-note.com/mcp`.

Native MCP OAuth 2.1 authorization is part of the HTTP release.

## Implementation Plan

1. Add the official MCP Go SDK with a pinned version.
   - Use Dockerized Go tooling.
   - Update `/Users/runming/workspace/tag_note/go.mod` and
     `/Users/runming/workspace/tag_note/go.sum`.

2. Add `/Users/runming/workspace/tag_note/internal/mcpoauth/`.
   - Publish protected-resource metadata.
   - Publish authorization-server metadata.
   - Support dynamic client registration for public PKCE clients.
   - Implement browser login, consent, code exchange, refresh rotation, and
     bearer-token verification.
   - Store only token/code hashes in SQLite.

3. Add `/Users/runming/workspace/tag_note/internal/mcpserver/`.
   - Load config from environment and flags.
   - Construct an MCP server for Streamable HTTP.
   - Register read tools first.
   - Add output caps and consistent service error mapping.

4. Add write tools.
   - Register write tools only when not read-only.
   - Register delete tools only when `TAGNOTE_MCP_ALLOW_DELETE=1`.
   - Implement `tagnote_set_note_pinned` idempotently.

5. Add resources and prompts.
   - Reuse the same service methods as tools.
   - Keep resources read-only and capped.
   - Add prompts once the tool names and schemas settle.

6. Add `/Users/runming/workspace/tag_note/cmd/tagnote-mcp/main.go`.
   - Default to HTTP on `:3001`.
   - Require MCP OAuth bearer authorization per HTTP request.

7. Wire build and docs.
   - Add `tagnote-mcp` to `/Users/runming/workspace/tag_note/Dockerfile`.
   - Document configuration in `/Users/runming/workspace/tag_note/README.md`.
   - Add local and production Compose services.
   - Route production through Caddy at `https://mcp.tag-note.com/mcp`.

8. Verify.
   - Run `docker build --target test .`.
   - Run `docker compose build tagnote-mcp`.
   - Smoke-test local metadata, DCR, and bearer challenge endpoints.

## Test Plan

- E2E-test native OAuth dynamic registration, browser-login approval, code
  exchange, bearer verification, and refresh rotation.
- E2E-test MCP over Streamable HTTP with bearer auth:
  - create a note through MCP
  - search it through MCP
  - update tags through MCP
  - pin/unpin through MCP
  - read a resource through MCP
  - reject unauthenticated `/mcp` requests with protected-resource metadata
- Confirm read-only mode exposes no write tools.
- Confirm content caps truncate large responses and report truncation metadata.
- Confirm unauthenticated HTTP MCP requests fail and authenticated requests can
  list tools.

## Acceptance Criteria

- An MCP host can connect to `https://mcp.tag-note.com/mcp` and list TagNote
  tools.
- With a valid bearer token, the host can search, read, create, and update notes
  without direct database access.
- Missing/invalid tokens fail clearly without leaking token values.
- Read-only mode prevents write tool registration.
- Delete tools are absent unless `TAGNOTE_MCP_ALLOW_DELETE=1`.
- Existing CLI tools keep working.
- `docker build --target test .` passes.

## Open Questions

- Should TagNote add an idempotent HTTP endpoint for pin state instead of MCP
  read-then-toggle behavior?
- Should image upload be exposed later as a tool, or should MCP only work with
  Markdown text and existing image URLs?

## References

- Model Context Protocol server concepts:
  https://modelcontextprotocol.io/docs/learn/server-concepts
- Model Context Protocol architecture and transports:
  https://modelcontextprotocol.io/docs/learn/architecture
- Official MCP SDK list:
  https://modelcontextprotocol.io/docs/sdk
- Official Go SDK:
  https://github.com/modelcontextprotocol/go-sdk
