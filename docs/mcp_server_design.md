# TagNote MCP Server Design

This document designs a Model Context Protocol server that lets an LLM read and
act on a user's TagNote data through the existing TagNote API.

## Goals

- Let an LLM search, read, create, update, tag, pin, and organize notes.
- Reuse TagNote's current HTTP API, JWT auth, service behavior, validation,
  audit logging, and deployment model.
- Keep the first release safe for local desktop MCP hosts by using stdio and an
  explicit user-provided TagNote token.
- Avoid direct SQLite access from the MCP process in the first release.
- Keep destructive actions opt-in and leave permanent deletion out of the
  default tool surface.

## Non-Goals

- Do not expose admin APIs, account deletion, operational status, metrics, or
  raw database access.
- Do not build an LLM into TagNote. The MCP server only exposes TagNote context
  and actions to MCP clients.
- Do not add UI changes to the web app or iOS app for the first release.
- Do not implement OAuth for remote MCP transport in the first release.

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
stdio
```

Required environment:

| Variable | Purpose |
| --- | --- |
| `TAGNOTE_URL` | TagNote base URL, defaulting to `http://localhost:3000` to match existing CLI behavior inside the container. |
| `TAGNOTE_TOKEN` | User JWT from `tagnote-login` or the app login flow. Required for all user data access. |

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

- The MCP server acts with the exact permissions of `TAGNOTE_TOKEN`.
- The token must not be logged.
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

### Local Desktop MCP Host

Build the binary into the existing Docker image and also allow local extraction
from the image.

Example MCP host command:

```json
{
  "command": "docker",
  "args": [
    "compose",
    "exec",
    "-T",
    "-e",
    "TAGNOTE_URL=http://tagnote:3000",
    "-e",
    "TAGNOTE_TOKEN",
    "tagnote",
    "tagnote-mcp"
  ],
  "env": {
    "TAGNOTE_TOKEN": "<user jwt>"
  }
}
```

For users running the binary outside Docker:

```bash
TAGNOTE_URL=https://tag-note.com TAGNOTE_TOKEN=<jwt> tagnote-mcp
```

### Future Remote Transport

After stdio is stable, add Streamable HTTP transport at a separate endpoint or
process. The preferred design is:

- `tagnote-mcp -transport http -addr :3001`
- Require `Authorization: Bearer <TagNote JWT>` on MCP HTTP requests.
- Validate JWTs using `/api/v1/auth/me` or shared `AuthService`.
- Keep per-request user scoping from the bearer token, not from tool arguments.
- Add Caddy routing only after auth and origin behavior are tested.

This should be a second phase because remote MCP authorization has more
deployment and client-compatibility risk than local stdio.

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
   - Construct an MCP server with stdio transport.
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
   - Default to stdio.
   - Fail fast when `TAGNOTE_TOKEN` is missing.
   - Print diagnostics to stderr only, never stdout, because stdout is MCP
     protocol traffic for stdio transport.

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
- Confirm stderr/stdout separation by running the stdio server through an MCP
  client or inspector.

## Acceptance Criteria

- An MCP host can connect over stdio and list TagNote tools.
- With a valid `TAGNOTE_TOKEN`, the host can search, read, create, and update
  notes without direct database access.
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
