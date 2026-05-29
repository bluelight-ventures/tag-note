# TagNote UX Guidelines

Cross-platform UX guidelines for TagNote. They apply to every client we ship
or will ship — web (`web/`), iOS (`ios/`), and any future Android, tablet,
desktop, or TV surface — and to the marketing site at `tag-note.com`.

One product, one feel, adapted to each device's input model and screen size.
Implementations reference these patterns; they do not override them. When a
rule and an implementation disagree, the rule is the spec — update the
implementation, or update this file with a deliberate reason and a date.

## Quick reference

- **[Foundations](#foundations)** — principles, voice, density, accessibility,
  performance.
- **[Architecture](#architecture)** — surfaces, navigation, adaptive behavior,
  onboarding.
- **[Components](#components)** — cards, chips, buttons, forms, modals,
  feedback.
- **[Flows](#flows)** — authoring, filtering, priority, manage, auth.
- **[Visual system](#visual-system)** — color, type, icons, spacing,
  elevation, motion.
- **[Quality bar](#quality-bar)** — state matrix, empty/loading/error,
  offline, privacy, marketing, **design review checklist**.

When two rules conflict, see [§33 Tradeoffs](#33-tradeoffs).

---

## Foundations

### 1. Implementation Rules

Short and load-bearing. Read these before reaching for any specific pattern.

1. **Tag-first.** Tagging is the only organizing primitive. Never add
   folders, notebooks, or hierarchies on any client.
2. **Four concepts only.** Write → Tag → Stream → Prioritize. Anything else
   is peripheral; tuck it into chrome (sidebar, settings, modals).
3. **Speed before features.** Interactions feel instant. Read paths never
   block on the network. Authoring saves in the background. See
   [§5 Performance perception](#5-performance-perception) for the budget.
4. **High information density.** TagNote is for people with a lot of notes
   and tags. Default to packed feeds, compact rows, and visible state.
   Density is a feature, not debt. See [§3](#3-information-density).
5. **Calm by default, loud only for priority.** Color signals are reserved
   for urgent / important / error. Everything else stays neutral.
6. **Adapt, don't duplicate.** The same surfaces reshape across phone,
   tablet, desktop, and TV. Never ship a feature only on one form factor.
7. **One product, one feel.** Same vocabulary, same priority semantics,
   same theme palette across every client.
8. **Themed, never hardcoded.** Every screen must hold up in all 8 themes
   (4 families × light/dark) and respect the OS theme by default.
9. **Trust is product.** No dark patterns. No required sign-up for trial.
   No tracking. Export is always one tap from primary chrome.
10. **Voice: terse, second-person, action-oriented.** "Tag your thinking.
    Find it instantly." Never marketing-ese.
11. **Accessible and offline by default.** Both are requirements, not
    enhancements. Every surface is operable with the platform's assistive
    input *and* does something useful when the network is gone.

### 2. Voice & tone

The product talks like a fast, helpful colleague — never a brand. Specific
rules so the voice survives many authors:

- **Verbs over nouns.** "Save note", not "Note saving". "Add tag", not
  "Tag addition".
- **Sentence case** for buttons, labels, menu items, and headings. Title
  Case only for proper nouns and product names (`TagNote`, `Google`,
  `Markdown`).
- **Second person, present tense.** "You're in guest mode." "We couldn't
  reach the server." Avoid "the user".
- **Numbers as digits.** "3 tags", "1 note", "12 min ago".
- **Always pluralize correctly.** Handle 0 / 1 / many. "1 tag" not
  "1 tags". Don't concatenate strings — use templates so translators have
  full sentences.
- **Dates.** Relative for ≤ 7 days ("just now", "5 min ago", "Yesterday",
  "Wednesday"); absolute beyond that ("May 26").
- **Errors:** name the problem, suggest a fix, don't blame. "We couldn't
  reach the server. Try again." beats "Error 500" or "Something went
  wrong".
- **Empty states:** explain the cause, suggest the next step. "No notes
  match the selected tags." beats "No results".
- **No exclamation marks in product UI.** The only celebratory emoji is
  `🎉` on the guest-limit screen. Marketing copy can be lightly warmer.
- **Microcopy bank** — prefer these exact strings where they fit:
  *Tag your thinking. Find it instantly.* · *New note* · *Save note* ·
  *Read more* · *Login* · *Create account* · *Login without password* ·
  *Try without an account* · *Open app*.

### 3. Information density

TagNote is built for users who maintain **many notes and many tags**. They
want to see context at a glance — not be guided through one item at a time.
Every product surface defaults to packed, compact, and stateful; whitespace
is earned, not free.

What this means in practice:

- **Pack the viewport.** Feeds render as multi-column masonry where the
  device allows. Cards size to content rather than to a fixed grid. The
  sidebar combines navigation, search, filter, and the tag cloud into a
  single scrollable column.
- **Tabular rows for management, not card lists.** Tags, trash, import
  previews, and any future "list of things" use compact rows with inline
  controls. One card per screen is for Focus, not for browsing.
- **State surfaces inline.** Chips, badges, and small color cues show
  counts, priority, and review status without opening another view — the
  unreviewed-tag count on the Tags nav item, the pinned-edge cue on cards,
  the priority left border, the saved/unsaved dot in Focus.
- **Chrome is on-demand.** Per-row and per-card actions reveal on hover /
  long-press / focus rather than occupying space full-time.
- **Show the long tail.** The tag cloud lists every tag with a "show all"
  affordance. Don't truncate aggressively when the user came here to find
  a specific tag.
- **Numbers are tabular.** Counts, priority values, and time deltas use
  tabular numerals so columns align for scanning.

**Acceptance signal:** on a 1440 × 900 desktop window, a power user with
50+ notes should see at least 12 note cards (titles + first lines) plus the
full tag cloud and active filters without scrolling.

What density does **not** mean:

- Not cluttered or unreadable. Body text stays at ≈ 14 sp with line-height
  1.5+. Headings still breathe within a card.
- Not unreachable. Touch targets remain ≥ 44 dp on touch, ≥ 48 dp at TV
  distance; pointer targets may shrink to ~28 dp for icon-only chrome.
- Not state-hiding. If a user must drill in to find out something basic
  (note count, last update time, urgent items, save state), density failed.

#### How density adapts

Density is a **scale**, not a per-device on/off.

| Form factor | What you see at once | What changes |
| --- | --- | --- |
| Phone | One column of compact cards, filter chips inline, badges visible. | Sidebar collapses behind a drawer. Never a "one feature per screen" walkthrough. |
| Tablet | 1–2 columns, collapsible side rail. | Hover-reveals become long-press. |
| Desktop | Persistent sidebar + 2-column masonry + visible filters + tag cloud. The reference layout. | Most state visible at once; secondary actions reveal on hover. |
| TV (10-foot) | Bigger cards, but a screen still shows many at once. | Focus rings make density navigable with a remote; type scales for viewing distance. |

**Anti-patterns** on any client:

- Modal walkthroughs, wizards, or carousels that force one step at a time.
- Hero-style empty space inside the product (marketing site is the
  exception — [§35](#35-marketing-site)).
- Hiding counts, status, or priority behind a tap when they fit on the
  primary surface.
- Pagination where infinite scroll under a sticky filter would expose more
  at once.

### 4. Accessibility & inclusive design

Treat accessibility as a baseline, not a separate ticket. Most rules here
are also good UX for everyone.

- **Color contrast.** Body text ≥ 4.5:1 against its surface; UI components
  and large text ≥ 3:1. Verify every theme variant.
- **Never encode meaning by color alone.** Pair color with shape, icon,
  position, or label (e.g., urgent = red *and* left border *and* "Urgent"
  label).
- **Focus visible.** A ≥ 2 px focus ring in `--accent` on every
  interactive element. Never `outline: none` without an equivalent
  replacement.
- **Hit targets.** Touch ≥ 44 dp; TV at distance ≥ 48 dp; pointer chrome
  may go to 28 dp. Increase, don't decrease, when in doubt.
- **Keyboard order matches visual order.** Tab through a surface in the
  order a reader would scan it.
- **Screen reader names** on every interactive element. Live regions
  announce save state, errors, and toasts.
- **Dynamic type / OS text size.** Respect the OS setting; don't cap below
  200% of default. No `font-size` smaller than 11 sp in chrome, 13 sp in
  body.
- **Reduced motion.** Replace non-essential animations with cross-fades
  when `prefers-reduced-motion` (or platform equivalent) is set. Save
  pulse, chip scale-in, and hero gradient all degrade gracefully.
- **RTL.** Mirror layouts for Arabic/Hebrew/Persian. Chips remain
  start-aligned. Icons that imply direction (back arrow, "read more"
  chevron) flip; icons that don't (trash, tag, pin) don't.
- **Internationalization.** No concatenated string assembly. Use
  templates with placeholders. Reserve 30 % extra width for translated
  strings on labels. Pluralize via ICU MessageFormat or the platform
  equivalent.
- **Locale.** Dates, numbers, and time formats follow the locale, not the
  app language.
- **Errors are textual.** Never communicate an error with color alone or
  via a tooltip.

### 5. Performance perception

Speed is a UX property, not just an engineering one. Hit these latency
budgets at the **perceived** level — they include layout, paint, and any
animation.

| Action class | Budget | UX rule |
| --- | --- | --- |
| Typing, hover, button press, chip add, filter toggle | < 100 ms | Feel instant. No spinner. 150 ms ease-out at most for the visual reaction. |
| Local navigation, sort, search-as-you-type | < 1 s | Acknowledge immediately. Inline progress only if > 500 ms. |
| First server fetch, login, sign-up | < 3 s | Show a spinner or skeleton; disable the action that's in flight. |
| Long upload, large export, import preview | > 3 s | Progress + cancel + recoverable failure. |

Other rules:

- **Optimistic UI** for safe, reversible actions: save, rename, reorder,
  toggle filter. Commit locally first; reconcile second. Authoring is the
  reference (see [§16 Authoring](#16-authoring-focus--save-state)).
- **Skeleton vs spinner.** Skeleton when the result shape is known
  (card grid, tag table row). Spinner for indeterminate single-action
  waits (login, export).
- **Don't show a spinner before 200 ms** — flicker is worse than waiting.
- **Never block the whole surface for a partial load.** Render what's
  cached or known; let the rest fill in.
- **No layout shift.** Reserve space for images, badges, and async chips so
  the feed doesn't reflow under the reader's eye.
- **First paint must not flash.** Restore theme before stylesheets resolve.

---

## Architecture

### 6. Surface model

Every client expresses the same four surfaces, presented natively. The
vocabulary is consistent; the chrome is not.

| Surface | Purpose | Web | iOS / mobile | TV |
| --- | --- | --- | --- | --- |
| **Stream** | Browse notes filtered by tags / search | Two-column masonry under a fixed sidebar | Vertical feed under a search/filter header; tab bar at bottom | Grid of cards, focusable with D-pad |
| **Focus** | Author or edit a single note | Full-screen overlay beside sidebar | Full-screen sheet | Full-screen with on-screen keyboard |
| **Read** | Render a single note | Overlay with rendered Markdown | Pushed detail view | Full-screen reader |
| **Manage** | Tags / Trash / Settings | Sidebar tabs + table panel | Tab bar destinations + lists | Side panel with focusable rows |

**Hierarchy rule.** One Focus surface visible at a time. Never nest Focus
inside Focus. A confirm modal *over* Focus is fine; another Focus is not.

### 7. Navigation & information architecture

- **Primary nav exposes exactly four destinations:** Notes (Stream), Tags
  (Manage), Trash, and the universal *New note* action. Anything else is
  secondary chrome (search, filter, theme, account, export).
- **Search and tag filter combine.** Search narrows by content; tag chips
  narrow by AND-membership. Both narrow the same feed.
- **Filtering is always reversible.** A single tap clears all chips; chips
  are visible while active.
- **Pinned items float to the top** of the Stream regardless of sort.
- **Sort options are intentionally minimal:** Newest first, Recently
  updated. Don't add more without a real user need.
- **State survives navigation.** Switching tabs preserves filter chips,
  search input, and scroll position.

### 8. Cross-platform consistency

What is **fixed** across every client:

- **Vocabulary.** Notes, Tags, Trash, Stream, Focus, Read, Importance,
  Urgency. Never rename these per platform.
- **Priority semantics.** Importance × Urgency lives on tags, not notes.
  Notes inherit from their highest-priority tag. Visual mapping is
  identical ([§18](#18-priority-importance--urgency)).
- **Density baseline.** A surface that shows N items on web shows roughly
  the same proportion of the viewport on iOS or TV. Don't ship a spacious
  mobile redesign that breaks the product's character.
- **Theme palette.** All 8 themes on every client. Same hex values; same
  token names where the platform supports them.
- **Tag chip behavior.** Commit-on-space/comma/enter, ghost styling for
  unknown tags, two-step removal.
- **Save semantics.** Background save; status surfaces
  Unsaved / Saving / Saved / Invalid / Failed.
- **Tone.** Same copy where the platform permits.
- **Data shape.** JSON export/import format is interchangeable. A web
  export imports into iOS unchanged.

What is **allowed to vary**:

- Container shape (sheet vs. overlay vs. push transition).
- Gestures (swipe-to-delete on iOS, hover-reveal on web, long-press on
  Android).
- Native iconography for **system** affordances (share, back, settings) —
  use SF Symbols on iOS, Material on Android. Branded glyphs stay
  identical.

### 9. Adaptive behavior

Same product, different gestures. When designing a feature, decide once how
it reshapes.

| Concern | Phone (≤ 480 dp) | Tablet (480–900 dp) | Desktop (≥ 900 dp) | TV / 10-foot |
| --- | --- | --- | --- | --- |
| Navigation | Bottom tab bar + drawer for filters | Collapsible side rail | Persistent sidebar (~280 dp) | Side rail, focus-ring driven |
| Feed columns | 1 | 1–2 | 2 (capped ~480 dp per column) | 3–4 grid |
| Primary action | Floating "+" / system compose | Toolbar button | Sidebar *New note* | D-pad-focusable card |
| Focus surface | Sheet covering full viewport | Modal beside rail | Overlay beside sidebar | Full-screen |
| Tag chip remove | Long-press | Long-press | Hover-reveal `×` | Focus chip → B |
| Hit targets | ≥ 44 dp | ≥ 44 dp | ≥ 28 dp for icon-only chrome | ≥ 48 dp at viewing distance |

Rules of thumb:

- **Breakpoint by content, not pixel-perfect.** Switch layouts when
  columns drop below ~280 dp or a side-by-side gets cramped.
- **Match the platform's primary gesture.** Pull-to-refresh on mobile,
  context menus on right-click, long-press on touch, A on TV.
- **Native components for system affordances** (keyboard, share sheet,
  file pickers, alerts) — replace only with a specific reason.
- **Both orientations.** Portrait and landscape both work on phones and
  tablets. Never lock orientation.
- **Foldables and split-view.** Focus is one region; Stream is the other.
  Keyboard input goes to Focus.

### 10. Onboarding & first-run

The product is small enough that the four concepts are self-explanatory
after one capture-tag-stream cycle. Don't ship a tour.

- **No mandatory walkthrough.** No carousel, no coachmarks chained
  together. A single tooltip is OK; a sequence is not.
- **First-run Stream** shows a nudge card: *Write your first note.* with a
  *New note* button. Dismissible. Disappears once any note exists.
- **First-run Tags** explains tags briefly with example text. No quiz, no
  forced approval.
- **Empty Trash** is honest: *Nothing deleted yet.*
- **Guest mode is the secondary onboarding cue.** The banner persists:
  *You're in guest mode. Notes are saved in this browser only.* with a
  *Create free account* CTA. The guest-limit screen is celebratory, never
  punitive.
- **No engagement nags.** No "you haven't written in 3 days" emails, no
  re-engagement push notifications, no streak gamification.

---

## Components

### 11. Cards

Notes, trash items, tag-management rows, and import-preview entries are
all **cards**: a padded rectangle, 1 px border, themed surface, 8 dp corner
radius. State is encoded on the card's **edges**, not its fill:

- **Pinned:** 2 px top edge in `--accent`.
- **Important:** 3 px left edge in amber.
- **Urgent:** 3 px left edge in red + faint red wash.
- **Trash:** 75 % opacity until focused/hovered.
- **Selected (TV / keyboard focus):** 2 px outline in `--accent`,
  offset −1 px.

Deterministic anatomy:

1. Top row — status chips (pin, priority) and tag chips.
2. Body — rendered Markdown; collapsed beyond ~300 dp with a fade and a
   *Read more* affordance.
3. Action row (revealed on hover / long-press / focus) — edit, delete, plus
   per-context actions.

If a new state needs a new visual, encode it on the edge or with a chip —
never by changing the card fill.

### 12. Tag chips

Used in Focus (author), Stream (filter), and Manage (rename). One behavior
across clients:

- **Space, comma, or Enter** commits the current text as a chip.
- Autocomplete shows existing tags with a priority dot and `I/U` scores.
- **Ghost chips** (dashed border, muted) indicate a tag that doesn't exist
  yet. Committing creates an *unreviewed* tag.
- **Two-step backspace** at empty input: first press highlights the
  trailing chip with the accent outline; second press removes it. (Touch:
  long-press the chip. TV: focus the chip, press B.)
- Chips colored by priority use `filter: brightness()` for hover so the
  priority hue is preserved.

### 13. Buttons & actions

Five roles. Pick exactly one **Primary** per surface.

| Role | Use |
| --- | --- |
| Primary | The one main action (Save note, Login). |
| Secondary | Neutral / opt-out (Cancel, Back). |
| Ghost | Icon-only chrome (sidebar header, card actions). |
| Danger | Destructive confirms only. |
| OAuth | Provider sign-in; brand mark intact. |

Rules:

- **Verb labels.** "Save note", "Create account", "Send reset link" —
  never just "OK" or "Submit".
- **Icon-only buttons always have an accessible name** (`aria-label`,
  accessibilityLabel, contentDescription) and a tooltip on pointer
  devices.
- **Loading state** disables interaction and appends a 12 dp spinner.
- **Disabled is a last resort.** Prefer letting the user click and showing
  why (see [§14 Forms](#14-forms--inputs)).

### 14. Forms & inputs

- **Labels sit above inputs**, never inside-only, in 13 sp / weight 500.
- **Focus state** borders to `--accent`; pair with a visible focus ring.
- **Always set autofill hints** — `autocomplete` on web,
  `textContentType` on iOS, `autofillHints` on Android — so password
  managers and platform autofill work.
- **Validation errors render inline** near the field; never hide behind a
  tooltip and never wait until submit if the field can be validated as
  the user types (e.g., email format).
- **Don't disable the submit button** to communicate "invalid". Let the
  user submit and tell them why. (Exception: a destructive button that
  would be irreversible if accidentally tapped.)
- **Password fields** include a show/hide toggle and a live strength bar
  on register / reset.
- **Inputs respect platform conventions:** Return-key labels, keyboard
  type, secure-text rendering, locale-aware date pickers.

### 15. Modals, sheets & inline feedback

Use a modal/sheet only for:

1. **Destructive confirm.**
2. **Prompt for a short input** (e.g., rename).
3. **A single celebratory or limit screen** (guest limit).

Anything else is its own surface.

Modal etiquette:

- Always include: an explicit dismiss path (Cancel + Esc / back / swipe
  down), a primary action, and the consequence in the **body** — never
  only the title.
- Destructive confirms use the Danger-styled button.
- Don't stack modals. If a second one is warranted, close the first.

Feedback channels:

| Channel | Use for | Example |
| --- | --- | --- |
| **Inline error** | User must act on this field now | "Email already in use." |
| **Save indicator** | Live status of authoring | Unsaved → Saving → Saved |
| **Toast** | Completed background action | "Note exported." |
| **Badge** | Persistent count that informs nav | Unreviewed tags |
| **Modal** | Destructive confirm / required choice | "Delete this note forever?" |
| **Empty state** | Container has nothing to show | "No notes match the selected tags." |

Toasts auto-dismiss within a few seconds; allow tap/click to dismiss
early. Never use a toast for an error the user must act on.

---

## Flows

### 16. Authoring (Focus + save state)

The single source of truth for *did the user lose work* lives here.

- **One Focus visible at a time.** Esc / back gesture / B button dismisses.
- **Unsaved changes prompt a confirm dialog before dismissal.** Never
  silently discard.
- **Save status indicator** is always present near the title and uses
  these five states with distinct color + iconography. Update **synchronously**
  with state changes — it must never lie.

| State | Meaning | Color cue |
| --- | --- | --- |
| Unsaved | Local changes pending | Amber |
| Saving | Request in flight (pulse) | Amber |
| Saved | Server acknowledged | Green |
| Invalid | Validation error | Red |
| Failed | Network/server error | Red |

- **Authoring is Markdown.** Editor exposes a small toolbar, side-by-side
  preview where space allows, paste/drag-drop/button for image uploads.
- See `design_docs/autosave_notes.md` for the autosave contract.

### 17. Filtering & search

- **Tag chips AND-combine.** A note must have *every* selected tag to
  appear.
- **Search and tag filter compose.** Both narrow the same feed.
- **Hits wrap in `<mark>`** with `--mark-bg` / `--mark-text`.
- **The tag cloud** shows every tag; a dedicated search narrows the cloud
  itself for users with many tags.
- A **Show all tags** affordance reveals the long tail when the cloud is
  truncated.

### 18. Priority (Importance × Urgency)

The only place in the product where two orthogonal sliders coexist.
Canonical:

- Two integers per tag: **Importance** (0–N) and **Urgency** (0–N).
- Notes inherit the **highest-priority tag's** values.
- Slider tracks are themed gradients — Importance: muted → green;
  Urgency: muted → amber → red.
- Card visual mapping is identical on every client ([§11](#11-cards)).
- **Focus surface** shows a live priority preview next to the tag input so
  the author sees the effect before saving.

### 19. Manage: tags, trash, settings

- **Tags** is a **dense table**, one row per tag: name, count, sliders for
  I/U, numeric I/U values, status (`unreviewed` amber / `approved` green),
  and per-row actions (approve, rename, delete). **Do not** redesign it
  as a card list — the table is the point.
- The Tags nav surfaces a badge with the count of unreviewed tags.
- **Trash** items at 75 % opacity, full opacity on focus/hover. Restore
  and permanently-delete actions; the latter is always confirm-gated.
- **Settings** (theme, export, import, account, logout) lives in the
  sidebar header on web and in the profile/account screen on mobile.
  Export and import are no more than two taps deep.

### 20. Authentication

Offer four paths in this order, on every client that supports them:

1. **Try without an account** (guest mode) — most prominent.
2. Email + password (with show/hide toggle).
3. **Login without password** (magic link).
4. Continue with Google (OAuth).

Other rules:

- Guest mode stores data locally and is clearly labelled.
- Password registration shows a live strength bar.
- **Inline errors only** — the user has nothing else to look at.
- Email verification, forgot-password, and reset-password are **full
  views**, not modals.

---

## Visual system

### 21. Color & theme system

8 themes ship on every client. Each is a complete set of design tokens
(surface, text, border, accent, semantic colors, code, shadow):

| Family | Feel |
| --- | --- |
| Everforest (default) | Warm, calm, earthy green/cream |
| Solarized | Muted, classic |
| Gruvbox | High-contrast retro warm |
| Nord | Cool, frosty blue-grays |

Rules:

- **Never hardcode color.** Use the platform's token system
  (CSS variables on web, semantic colors on iOS, theme attrs on Android).
- The OS light/dark preference selects the family's variant on first run.
- Theme changes apply instantly without reload.
- First paint must not flash — restore theme before stylesheets resolve.

**Semantic color usage is fixed across themes:**

| Token | Meaning |
| --- | --- |
| Accent | Primary action, active filter, focus ring |
| Green | Success / approved / saved |
| Blue | Links / info / rename |
| Red | Destructive / urgent / error |
| Amber | Warning / unsaved / important / search highlight |

Never invent a new semantic color without first confirming none of these
fits.

### 22. Typography & type scale

- **Use the platform's system font:** SF on Apple, Roboto on Android,
  Segoe on Windows, system stack on web. No web fonts — startup cost.
- **Code:** the platform monospace stack.
- **Tabular numerals** for counts, priority values, and time deltas.

Type scale (rem-equivalent; map to sp/dp per platform):

| Token | Size | Weight | Use |
| --- | --- | --- | --- |
| `display` | clamp(40, 6vw, 64) px | 800 | Marketing hero only |
| `h1` | 32–36 px | 800 | Section headers on marketing |
| `h2` | 20 px | 700 | Surface title (Focus title, Modal title) |
| `h3` | 17 px | 700 | Card headings, feature card titles |
| `body` | 14 px | 400/500 | Default text |
| `body-md` | 13 px | 400 | Sidebar entries, autocomplete |
| `caption` | 12 px | 500 | Timestamps, helper text |
| `label` | 11 px | 600, +0.05 em, UPPERCASE | Sidebar section titles, table headers |

Line-height: 1.5 for chrome, 1.75 for rendered Markdown. Letter-spacing
negative on display (≈ −1.5 px) and h1 (≈ −0.5 px).

### 23. Iconography

- 2 px stroke, rounded line caps and joins.
- 16–20 dp in chrome, 44–48 dp in feature surfaces.
- The **brand mark** (tag silhouette with a small hole, filled in
  `--accent`, 6 dp corners) always pairs with the wordmark in weight 800.
  Identical across clients — don't redraw per platform.
- Use platform-native glyphs (SF Symbols / Material) only for system
  affordances. Branded glyphs stay consistent.

### 24. Spacing scale

Base unit **4 dp**. Use only these tokens; never invent in-between values.

| Token | Value | Typical use |
| --- | --- | --- |
| `s-1` | 4 dp | Tight pairs (icon-to-text in a button, chip gap) |
| `s-2` | 8 dp | Compact-control padding, related items in a row |
| `s-3` | 12 dp | Card top/bottom padding, input vertical padding |
| `s-4` | 16 dp | Card side padding, gap between siblings in a panel |
| `s-5` | 24 dp | Between sibling sections within a surface |
| `s-6` | 32 dp | Above primary section headings |
| `s-7` | 48 dp | Above-fold hero padding (marketing) |
| `s-8` | 64 dp+ | Marketing-only large vertical rhythm |

If a layout needs 18 dp, round to 16 or 24. If it needs 22, you're
probably aligning to the wrong baseline.

### 25. Elevation & layering

Use elevation sparingly — TagNote leads with **borders**, not shadows.

| Layer | Web `z-index` | When |
| --- | --- | --- |
| In-flow surface | auto | Cards, panels |
| Sticky chrome | 90 | Mobile header |
| Persistent nav | 100 | Sidebar |
| Focus overlay | 150 | Authoring/reading overlay |
| Modal | 200 | Confirm/prompt/guest-limit |
| Toast | 300 | Transient feedback |
| Autocomplete popover | 50 | Above inputs, below sidebar |

On native, rely on the platform's stacking (sheets above nav, alerts above
sheets).

Shadow tokens map to elevation:

| Token | Use |
| --- | --- |
| `shadow-sm` | Hover/lift of a card or button |
| `shadow` | Focused overlay, autocomplete popover |
| `shadow-lg` | Modal, sheet, marketing demo window |

Never use shadow as a primary border replacement.

### 26. Motion budget

Animation is decoration, never a requirement. Respect
`prefers-reduced-motion` everywhere. The sanctioned palette:

| Use | Duration | Easing |
| --- | --- | --- |
| Hover / focus state change | 150 ms | ease |
| Theme transition | 200 ms | ease |
| Surface open / dismiss | 150–250 ms | ease-out |
| Chip mount / dismount | 150 ms | ease-out (scale + fade) |
| Save-status pulse (loop while saving) | 1 s | ease-in-out |
| Spinner | 600 ms loop | linear |
| Hero accent gradient (marketing only) | 10 s loop | ease-in-out |

Don't add new keyframes without a clear new state to express.

---

## Quality bar

### 27. State matrix

Every interactive component must define these states before it ships.
Multiple visual channels per state — never color alone.

| State | Required for | Visual encoding |
| --- | --- | --- |
| Default (resting) | All | Base tokens |
| Hover | Pointer only | Color + shadow/border shift |
| Pressed / active | Buttons, tabs, chips | Inset shadow or color darken |
| Focus visible | All | ≥ 2 px ring in `--accent`, never suppressed |
| Selected (toggle) | Filter chips, tabs, sort | `--accent` fill + role="…" + label |
| Disabled | Last resort | 50 % opacity + `cursor: not-allowed` + remove from tab order |
| Loading | Buttons, surfaces, async lists | Skeleton (if shape known) or spinner after 200 ms |
| Empty | Containers | Single-line muted copy with next step |
| Error | Inputs, surfaces | Inline text + red icon + retry path |
| Success | Save indicator, toast | Green + label/icon |

A component that doesn't declare these states isn't done.

### 28. Empty, loading, error

Always render *something*. A blank surface is a bug.

- **Empty.** Muted single line explaining the cause and, if relevant, the
  next step. *No notes match the selected tags.* beats *No results.*
- **Loading.** Skeleton if the result shape is known; spinner otherwise.
  Never block the whole surface for a partial load. Don't show a spinner
  before 200 ms.
- **Error.** Inline at the field level; toast for completed background
  failures; modal only when the user must decide. Always include a
  recovery path (retry, dismiss, contact).

### 29. Offline & sync

- **Read paths must work offline** on every client.
- **Authoring queues edits locally** and syncs when the network returns;
  the save-status indicator surfaces this state honestly.
- **Web ships as a PWA** — installable, service-worker cached, manifest
  configured. The install affordance lives in primary chrome.
- **Native clients** support background sync where the platform allows.
- **Conflicts** prefer the *most recently edited* version and offer the
  loser as a recoverable backup — never silently overwrite.

### 30. Privacy

- No third-party trackers in the product shell, on any client.
- Export (JSON) is reachable from primary chrome, not buried in settings.
- Guest mode clearly labels local-only storage.
- All four auth paths coexist; never force one over another silently.
- Don't ship telemetry that the user can't see.

### 31. Marketing site

The marketing site at `tag-note.com` shares tokens with the app but is
**not** themable — visitors get one consistent look (Everforest Light). It
is the **only TagNote surface where generous whitespace is the default**;
the product stays dense ([§3](#3-information-density)).

Section order is load-bearing:

1. **Hero** — animated headline, two CTAs (*Get started free* +
   *Try it now — no sign-up*).
2. **Interactive demo** — clickable tag chips filtering an example feed.
   This is the product's hero shot.
3. **Priority showcase** — Importance × Urgency explained.
4. **Features grid** — 6 cards: tags, search, Markdown, themes, PWA,
   portability.
5. **How it works** — Write → Tag → Stream → Prioritize.
6. **CTA repeat.**
7. **Footer** — brand + Privacy + Terms.

Don't introduce a build step; inline CSS stays in `<style>`. Lead every
section with one headline and one sub-headline; no multi-paragraph
intros.

### 32. Design review checklist

Before merging any new surface or component, verify every item:

- [ ] Renders correctly in **all 8 themes** (4 families × light/dark).
- [ ] Works at the **narrowest supported width** (320 dp) and the widest
      (≥ 1920 dp); both orientations on touch devices.
- [ ] Pointer, touch, keyboard, screen reader, and D-pad can all complete
      the primary task.
- [ ] **Voice** is terse, second-person, sentence-case; pluralization,
      dates, and numbers respect locale.
- [ ] **Color is not the only signal.** Each state has an additional cue
      (icon, label, position, border).
- [ ] **Hit targets** meet the floor (28 / 44 / 48 dp by input class).
- [ ] **All states** from [§27](#27-state-matrix) are defined: default,
      hover, pressed, focus, disabled (if any), selected, loading, empty,
      error, success.
- [ ] **Loading** uses skeleton when the result shape is known; spinner
      only after 200 ms.
- [ ] **Errors include a recovery path** and are textual.
- [ ] **Offline read paths still work; offline write paths queue** and
      surface in the save indicator.
- [ ] **No new keyframes** outside the sanctioned animation palette
      ([§26](#26-motion-budget)).
- [ ] **Density tier matches the form factor.** No spread-out card list
      where a table fits.
- [ ] **No hardcoded color, spacing, or shadow values.** Every value maps
      to a token.
- [ ] **RTL flipped** correctly; directional icons mirror.
- [ ] **Filtering, search, and pin order** behave as
      [§17](#17-filtering--search) and [§11](#11-cards) expect.

---

## Reference

### 33. Tradeoffs

Principles sometimes pull against each other. When they do, apply these
explicitly:

| Tension | Rule |
| --- | --- |
| **Density vs touch target** | Density yields. Never shrink a touch target below 44 dp to fit more on a phone. |
| **Speed vs trust** | If an action is cheaply reversible, prefer **instant + undo toast** over a confirm modal. If it can't be reversed (permanent delete, export to third party), use a confirm modal with a danger-styled button. |
| **Adapt vs native** | Use native components for **system** affordances (keyboard, file picker, share sheet, alerts). Use TagNote patterns for **product** behavior (chips, priority, save status). |
| **Consistency vs platform idiom** | Vocabulary and semantics are global. Container shape and gesture are local. |
| **Information vs onboarding** | Information always wins. We do not hide an inhabitable surface behind an empty-state coachmark for a new user. |

### 34. See also

- `design_docs/autosave_notes.md` — autosave contract behind the
  save-status indicator.
- `design_docs/ux_principles.md` — short decision-making brief for TagNote
  design work.
- `design_docs/ios_app_design.md` — companion iOS app direction.
- `web/style.css` — canonical design tokens for the web client.
- `web/index.html`, `web/landing.html` — web reference implementations of
  the surfaces and chrome described above.
