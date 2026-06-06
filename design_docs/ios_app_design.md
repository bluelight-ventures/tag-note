# TagNote iPhone App Design

## Goal

Build a native iPhone app for TagNote that uses the same `/api/v1` backend as
the web app and keeps the product feel familiar: notes first, tag-driven
filtering, Markdown editing, quiet autosave, and the same theme language.

The first version should be an iPhone-focused app, not a universal iPad/Mac
client. It should work against a TagNote server by letting the user enter a
server URL.

## Recommended Tech Stack

Use native SwiftUI.

- App UI: SwiftUI, iOS 17+ target.
- Navigation: `NavigationStack`, `TabView`, sheets, and full-screen editor
  views.
- Networking: Swift `URLSession` with async/await.
- Auth storage: Keychain for the JWT token and selected server URL.
- Local cache: SQLite through GRDB, or SwiftData if we accept a looser match to
  the server schema.
- Markdown rendering: `MarkdownUI` for preview.
- Markdown editing: start with SwiftUI `TextEditor` plus a small formatting
  toolbar; evaluate a richer editor later if needed.
- Image picking/upload: `PhotosPicker` and multipart upload to
  `POST /api/v1/images`.
- Testing: XCTest for API/client/storage logic; XCUITest for auth, note CRUD,
  filtering, and offline/refresh flows.

Native SwiftUI is the best default because the app needs good iOS behaviors:
Keychain, keyboard handling, share sheet/import/export, image picker, offline
cache, and long-term App Store maintainability.

## Alternatives Considered

### React Native

Useful if we expect Android soon and want one mobile codebase. The downsides are
extra runtime dependencies, more moving pieces for Markdown editing, and less
direct access to polished iOS system behavior. It is reasonable only if Android
is a near-term requirement.

### Capacitor/WKWebView Wrapper

Fastest path because the current PWA already exists. It would not deliver a
meaningfully native iPhone app, and the current web sidebar/editor interactions
would still need heavy mobile-specific polish. This is a fallback prototype
path, not the recommended product path.

### Shared Kotlin Multiplatform Core

Not worth the setup cost for an iPhone-only first version. It may make sense
later if mobile clients need a shared offline sync engine.

## Existing API Surface

The app can use the current web API without a mobile-specific backend.

Auth:

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/google`
- `POST /api/v1/auth/verify-email`
- `POST /api/v1/auth/resend-verification`
- `POST /api/v1/auth/forgot-password`
- `POST /api/v1/auth/reset-password`
- `POST /api/v1/auth/magic-link`
- `POST /api/v1/auth/verify-magic-link`

Notes:

- `POST /api/v1/notes`
- `GET /api/v1/notes?tag=...&q=...&sort=...&limit=...&offset=...`
- `GET /api/v1/notes/:id`
- `PUT /api/v1/notes/:id`
- `PUT /api/v1/notes/:id/pin`
- `PUT /api/v1/notes/:id/restore`
- `DELETE /api/v1/notes/:id`
- `DELETE /api/v1/notes/:id/permanent`
- `GET /api/v1/notes/trash`
- `GET /api/v1/notes/export`
- `POST /api/v1/notes/import`

Tags:

- `GET /api/v1/tags`
- `GET /api/v1/tags/detailed`
- `GET /api/v1/tags/autocomplete?q=...&limit=...`
- `PUT /api/v1/tags/approve-all`
- `PUT /api/v1/tags/:name/approve`
- `PUT /api/v1/tags/:name/rename`
- `PUT /api/v1/tags/:name/priority`
- `DELETE /api/v1/tags/:name`

Other:

- `POST /api/v1/images`
- `GET /api/v1/settings`
- `PUT /api/v1/settings`

## Backend Gaps To Decide Before Implementation

No backend change is required for the MVP, but these would improve the native
experience:

- Deep-link targets for email verification, password reset, and magic-link
  login, for example `tagnote://auth/magic-link?token=...`.
- Optional refresh-token/session revocation model. The current API uses a JWT
  and logout is effectively client-side.
- Lightweight sync metadata endpoint if offline editing becomes first-class.
  The current list/update endpoints are enough for online-first with cache.
- Explicit server capability/config endpoint so the app can know whether Google
  login, email verification, and magic links are configured.

## Product Scope

### MVP

- Connect to a TagNote server URL.
- Login, register, forgot password, magic-link request, email verification
  pending state, and logout.
- List notes with search, tag filters, pagination, and sort.
- Create, edit, autosave, delete, restore, permanently delete, and pin notes.
- Manage note tags during editing with autocomplete.
- View and manage tags: approve, rename, delete, and edit importance/urgency.
- Upload images from the photo library into notes.
- Settings for theme and preview mode.
- Local read cache for recent notes and tags.

### Later

- Full offline editing with conflict handling.
- Home screen quick action for new note.
- Widgets for pinned or recent notes.
- Import/export through the iOS document picker.
- iPad split view.
- Android client.

## App Architecture

Use a small layered structure that mirrors the backend shape:

- `TagNoteAPI`: typed HTTP client around `/api/v1`.
- `AuthStore`: Keychain-backed token, server URL, and current user.
- `NoteRepository`: combines API fetches with local cache.
- `TagRepository`: caches detailed tags and autocomplete results.
- `SettingsRepository`: loads and saves app/server preferences.
- `SyncQueue`: only needed when offline editing is added.
- `ViewModels`: SwiftUI state for Notes, Editor, Tags, Trash, and Auth.

The API client should be generated manually at first. The API surface is small,
and hand-written request/response models make it easier to match current JSON
exactly.

## Data Model

Mirror the backend JSON:

- `SubNote`: `id`, `short_id`, `content`, `snippet`, `created_at`,
  `updated_at`, `tags`, `pinned`.
- `TagInfo`: `name`, `status`, `note_count`, `importance`, `urgency`.
- `Settings`: `theme`, `preview_mode`, `note_width`.
- `User`: `id`, `email`, `display_name`, `created_at`, `email_verified`,
  `has_password`, `has_google`.

Use `short_id` for routes and UI identity where the web app does. Keep `id`
available for future sync logic.

## UI Design

The web app is sidebar-first on desktop. On iPhone, convert that into bottom
tabs plus sheets:

- Notes tab: feed, search, active tag chips, sort menu, new-note button.
- Tags tab: tag management list with status, count, and priority controls.
- Trash tab: deleted notes with restore and permanent delete actions.
- Settings tab: account, server, theme, preview mode, import/export later.

### Visual Language

Reuse the web theme tokens:

- Default Everforest light background.
- Card surface for notes.
- Accent green for primary actions and active tags.
- Priority borders/colors based on tag importance and urgency.
- Rounded corners around 8px, restrained shadows, dense note-first layout.

Native controls should feel iOS-native, but colors, spacing, and information
hierarchy should clearly match the web app.

### Notes Feed

Layout:

- Top navigation bar: `TagNote`, search button, filter button.
- Search field expands at the top of the feed.
- Active tag filters appear as horizontal chips under search.
- Sort appears in a menu: newest first, recently updated.
- Feed rows are note cards with content preview, tag chips, timestamp, pin
  state, and swipe actions.
- Floating or toolbar new-note button.

Interactions:

- Tap card opens editor.
- Swipe right pins/unpins.
- Swipe left deletes.
- Tap tag chip toggles that filter.
- Pull to refresh reloads notes and tags.
- Infinite scroll uses `limit` and `offset`.

### Editor

Use a full-screen editor similar to the web focus editor.

Header:

- Close button.
- Title: `New note` or `Edit note`.
- Autosave status: `Unsaved`, `Saving...`, `Saved`, `Save failed`.
- More menu: delete, pin/unpin, preview mode.

Body:

- Tag chip input at top with autocomplete.
- Markdown text editor.
- Formatting toolbar above the keyboard: bold, italic, heading, list, quote,
  link, image.
- Preview segmented control: write, preview.

Autosave:

- Same behavior as web autosave: debounce edits, require content and at least
  one tag before creating a new note, preserve unsaved changes on failure.
- For existing notes, `PUT /notes/:id`.
- For new notes, `POST /notes` once valid, then switch to edit mode with the
  returned `short_id`.

### Tags

Layout:

- Search tags field.
- Segmented filter: all, unreviewed, approved.
- Tag rows show name, note count, status, and priority color.
- Expanded row exposes importance and urgency sliders.

Actions:

- Approve.
- Rename.
- Delete.
- Approve all in toolbar/menu.

### Trash

Layout:

- Note cards similar to the feed, visually muted.
- Actions: restore, delete forever.

### Auth

First launch:

1. Server URL screen.
2. Login/register screen.
3. Optional pending email verification screen.

Support password login first. Magic link can request a link immediately, but
token verification needs universal links or custom URL scheme work to feel
native. Google sign-in should use the iOS Google Sign-In SDK and then submit the
ID token to `POST /api/v1/auth/google`.

## Share Extension

Other apps share content into TagNote through a Share Extension target
(`TagNoteShare`, bundle `com.tag-note.tagnote.Share`) that creates a note from
the shared item. It activates for web URLs, plain text, and images.

**How a shared web page looks.** A page is captured as a compact, link-first
note rather than the full document, so even a huge site is a small, clean card:

```
# Page Title

[host/path](https://full/url)

> the text you had selected (if any)
```

The first `#` heading becomes the card title and the link is tappable. A
**Link / Full page** toggle in the compose screen lets the user instead capture
the page's readable text. Full-page text is only available for Safari shares
(extracted by a JavaScript preprocessing file, `GetPageInfo.js`, which returns
the title, canonical URL, selection, and `document.body.innerText`); v1 stores
that text as plain paragraphs (richer HTML→Markdown is a later enhancement). A
plain `public.url` shared from a non-Safari app has no page DOM, so it is
link-only. Plain text becomes the note body; images are downscaled and uploaded,
then embedded as `![](path)`.

**Compose UX.** The extension shows a small compose sheet — editable content, a
tag-chip input, the Link/Full-page toggle, and Cancel/Post — mirroring the
tag-first app. Untagged posts get the reserved `$default` chip. Posting reuses
`TagNoteAPI.createNote` (and `uploadImage` for images).

**Auth sharing.** The extension reads the session (token + server URL) from a
shared keychain access group (`$(AppIdentifierPrefix)com.tag-note.tagnote.shared`,
listed in both the app and extension entitlements). The app writes credentials
into this group on login; a one-time migration in `SessionStore` moves
pre-existing sessions over. When no session is present, the extension shows a
"sign in" prompt. Release builds default the server to `tag-note.com`; Debug
builds reuse the custom server the app stored.

**Code reuse.** `TagNoteAPI`, `KeychainStore`, and the model DTOs are compiled
into the extension target directly; heavyweight app code (AppState, the SwiftUI
app views, GoogleSignIn) is not. Pure pieces (`ShareMarkdownBuilder`,
`SharePayload` extraction) are unit-tested in `TagNoteTests`.

## Offline Strategy

Start online-first with cache:

- Cache notes, detailed tags, settings, and current user after successful
  fetches.
- Show cached content when offline.
- Disable edits while offline for MVP, or allow local drafts only inside the
  editor until connectivity returns.

Full offline editing should be a later milestone with:

- Local mutation queue.
- Per-note sync state.
- Conflict detection using `updated_at`.
- A conflict UI that lets the user keep local, keep remote, or duplicate.

## Security

- Store JWT in Keychain, not `UserDefaults`.
- Store server URL in Keychain or protected app storage.
- Require HTTPS for non-localhost servers by default.
- Redact token values in logs.
- Use App Transport Security defaults; document how operators should serve
  the API over HTTPS.

## Implementation Plan

1. Create an `ios/TagNote` SwiftUI project.
2. Add API models and `TagNoteAPI`.
3. Implement server URL, login, register, token storage, and `/auth/me`.
4. Build notes feed with cache and pull-to-refresh.
5. Build editor with tags, autosave, and image upload.
6. Build tag management.
7. Build trash.
8. Add settings, themes, and preview mode.
9. Add tests and App Store packaging assets.

## Open Questions

- Should the app require iOS 17+, or support iOS 16?
- Do we want offline editing in v1, or only cached reading?
- Should guest mode exist on iPhone? The web guest mode is browser-local demo
  behavior, while an iPhone app should probably start with server login.
- Which Markdown editor quality bar is acceptable for v1: native `TextEditor`
  plus toolbar, or a richer editor using an embedded web editing component?
- Should the app be distributed only through TestFlight/App Store, or
  should we also support sideloading/developer builds for personal servers?
