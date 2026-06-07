# TagNote MCP Server Design

This document designs a Model Context Protocol server that lets an LLM read and
act on a user's TagNote data through the existing TagNote API.

## Goals

- Let an LLM search, read, create, update, tag, pin, and organize notes.
- Reuse TagNote's current HTTP API, JWT auth, service behavior, validation,
  audit logging, and deployment model.
- Use the same long-running HTTP MCP interface locally and in production.
- Avoid direct SQLite access from the MCP process in the first release.
- Keep destructive actions opt-in and leave permanent deletion out of the
  default tool surface.

## Non-Goals

- Do not expose admin APIs, account deletion, operational status, metrics, or
  raw database access.
- Do not build an LLM into TagNote. The MCP server only exposes TagNote context
  and actions to MCP clients.
- Do not add UI changes to the web app or iOS app for the first release.
- Do not claim native MCP OAuth 2.1 authorization support until the browser
  login and callback flow is implemented.

## Current TagNote Integration Points

The implementation should start from these existing paths:

| Path | Role |
| --- | --- |
| `/Users/runming/workspace/tag_note/internal/apiclient/client.go` | Existing authenticated HTTP client used by CLI tools. Extend this first. |
| `/Users/runming/workspace/tag_note/internal/model/model.go` | Shared request/response and domain structs. |
| `/Users/runming/workspace/tag_note/internal/service/service.go` | Business behavior behind notes, tags, trash, import/export, and settings. |
| `/Users/runming/workspace/tag_note/internal/handler/handler.go` | Existing HTTP route behavior and status mapping. |
| `/Users/runming/workspace/tag_note/cmd/tagnote-login/main.go` | Existing token bootstrap flow for local users. |
| `/Users/runming/workspace/tag_note/Dockerfile` | Canonical build/test path and final image binary list. |

The first MCP server should be an HTTP-backed adapter, not a repository-backed
adapter. That keeps one write path for all clients and preserves the audit
middleware already registered on protected HTTP routes.

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
| `TAGNOTE_URL` | TagNote base URL, defaulting to `http://localhost:3000` to match existing CLI behavior inside the container. |

Until native MCP authorization is implemented, HTTP clients can pass a user
token per request:

```http
Authorization: Bearer <TagNote JWT>
```

The intended production design is native MCP Authorization Specification support
so compatible MCP clients can initiate a connection, discover the auth server,
open a browser login flow, and receive the token callback automatically.

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
  "resource_signing_alg_values_supported": ["RS256"]
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

- Use asymmetric signing, preferably `RS256`, so MCP can validate via JWKS.
- Include `iss: "https://mcp.tag-note.com"`.
- Include `aud: "https://mcp.tag-note.com/mcp"`.
- Include `sub: <TagNote user ID>`.
- Include `client_id`.
- Include `scope`.
- Expire quickly, for example 15 minutes.

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

- signature using current JWKS
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

The bearer-JWT shortcut can remain in development only behind an explicit
`TAGNOTE_MCP_ALLOW_LEGACY_JWT=1` flag. Production should require MCP-issued
OAuth access tokens once native auth is implemented.

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

The resource and tool implementations should share the same client methods and
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

## HTTP Client Work

Refactor `/Users/runming/workspace/tag_note/internal/apiclient/client.go` before
adding MCP handlers:

- Add a `Client` struct with `BaseURL`, `Token`, `HTTPClient`, `UserAgent`, and
  default timeout.
- Add context-aware methods:
  - `CreateNote`
  - `ListNotes`
  - `GetNote`
  - `UpdateNote`
  - `DeleteNote`
  - `RestoreNote`
  - `TogglePin`
  - `ListTrashed`
  - `ListTags`
  - `ListTagsDetailed`
  - `AutocompleteTags`
  - `ApproveTag`
  - `RenameTag`
  - `DeleteTag`
  - `UpdateTagPriority`
  - `GetSettings`
  - `SaveSettings`
  - `RenderStream`
- Preserve existing package-level functions so current CLI tools keep working.
- Fix status handling consistently. Some existing methods decode responses
  without checking non-2xx status first; MCP should not inherit that behavior.

## Server Package Design

Add `/Users/runming/workspace/tag_note/internal/mcpserver/` with:

```text
config.go      environment and flag parsing
server.go      MCP server construction and tool/resource registration
tools.go       tool handlers
resources.go   resource handlers and URI parsing
prompts.go     prompt definitions
caps.go        result size caps and note redaction helpers
errors.go      TagNote/API errors to MCP tool errors
```

Core types:

```go
type Config struct {
    BaseURL         string
    Token           string
    ReadOnly        bool
    AllowDelete     bool
    MaxNotes        int
    MaxContentBytes int
}

type Server struct {
    cfg    Config
    client *apiclient.Client
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
- Set `User-Agent: tagnote-mcp/<version>` on API requests so audit logs can
  identify MCP activity.
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
- Require `Authorization: Bearer <TagNote JWT>` on MCP HTTP requests.
- Validate JWTs using `/api/v1/auth/me` or shared `AuthService`.
- Keep per-request user scoping from the bearer token, not from tool arguments.
- Route production through Caddy at `https://mcp.tag-note.com/mcp`.

Native MCP OAuth 2.1 authorization must be part of the HTTP release, not a later
compatibility patch.

## Implementation Plan

1. Add the official MCP Go SDK with a pinned version.
   - Use Dockerized Go tooling.
   - Update `/Users/runming/workspace/tag_note/go.mod` and
     `/Users/runming/workspace/tag_note/go.sum`.

2. Refactor the API client.
   - Introduce context-aware `apiclient.Client`.
   - Add missing note/tag/settings/trash methods.
   - Preserve the existing CLI helper functions.
   - Add unit tests with `httptest`.

3. Add `/Users/runming/workspace/tag_note/internal/mcpserver/`.
   - Load config from environment and flags.
   - Construct an MCP server with Streamable HTTP transport.
   - Register read tools first.
   - Add output caps and consistent API error mapping.

4. Add write tools.
   - Register write tools only when not read-only.
   - Register delete tools only when `TAGNOTE_MCP_ALLOW_DELETE=1`.
   - Implement `tagnote_set_note_pinned` idempotently.

5. Add resources and prompts.
   - Reuse the same client methods as tools.
   - Keep resources read-only and capped.
   - Add prompts once the tool names and schemas settle.

6. Add `/Users/runming/workspace/tag_note/cmd/tagnote-mcp/main.go`.
   - Default to HTTP on `:3001`.
   - Require bearer authorization per HTTP request.

7. Wire build and docs.
   - Add `tagnote-mcp` to `/Users/runming/workspace/tag_note/Dockerfile`.
   - Document configuration in `/Users/runming/workspace/tag_note/README.md`.
   - Add a short MCP section to `/Users/runming/workspace/tag_note/TESTING.md`.

8. Verify.
   - Run `docker build --target test .`.
   - Run `docker compose build`.
   - Run local API smoke tests through `tagnote-mcp` against
     `TAGNOTE_TEST_MODE=1`.
   - Run MCP Inspector from Docker if Node tooling is needed.

## Test Plan

- Unit-test `apiclient.Client` status handling, auth header behavior, query
  encoding, JSON decoding, and context cancellation.
- Unit-test MCP tool handlers with a fake API client interface.
- Integration-test against a running TagNote container:
  - login as `test@test.com`
  - create a note through MCP
  - search it through MCP
  - update tags through MCP
  - pin/unpin through MCP
  - soft-delete only when delete is explicitly enabled
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

- Should `tagnote-login` gain a non-interactive `--print-token` or
  `--json` mode to make MCP setup easier?
- Should TagNote add an idempotent HTTP endpoint for pin state instead of MCP
  read-then-toggle behavior?
- Should a future remote MCP endpoint use TagNote JWTs directly or add a
  narrower MCP-scoped token type?
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
