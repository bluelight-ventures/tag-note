# TagNote UX Principles

This is the short decision-making brief for TagNote design work. Use it before
opening mocks, editing CSS, changing flows, or adding a new client surface.

For detailed component rules, state matrices, visual tokens, and cross-platform
behavior, see [`ux_guidelines.md`](ux_guidelines.md). When this file and the
guidelines appear to disagree, treat this file as intent and
`ux_guidelines.md` as implementation detail; update both when the product
direction changes.

## Product Promise

TagNote helps people capture many small pieces of thinking, tag them quickly,
and find them again without maintaining folders or notebooks.

The product should feel:

- **Fast** enough that capture never feels like a chore.
- **Dense** enough that a large note collection remains visible and scannable.
- **Quiet** enough that writing stays central.
- **Trustworthy** enough that users believe their notes are saved, portable,
  private, and recoverable.

## Core Principles

### 1. Tags are the structure

Tags are the only organizing primitive. Do not add folders, notebooks,
workspaces, nested collections, or parallel taxonomies.

Good TagNote design makes tags useful everywhere:

- Authoring makes tags quick to add and revise.
- Browsing makes active tag filters visible and reversible.
- Management makes tag state, counts, and priority easy to scan.
- Search and tags narrow the same stream instead of becoming separate modes.

If a proposed feature needs a hierarchy, first ask whether tags, search, pinning,
or priority can solve the user need with less structure.

### 2. Keep the product to four concepts

The durable product model is:

1. **Write** a note.
2. **Tag** it.
3. **Stream** the collection.
4. **Prioritize** through tag importance and urgency.

Everything else is support: settings, import/export, account, trash, theme,
uploads, onboarding, and admin behavior. Support features should stay in chrome,
menus, tables, or dedicated management surfaces. They should not compete with
the core loop.

### 3. Speed is a feature

Capturing a thought, applying a tag, filtering the stream, and returning to a
note should feel immediate. Save work in the background, acknowledge actions
quickly, and render known local state before waiting for the network.

Speed does not mean hiding truth. If a save is pending, failed, queued offline,
or invalid, show that state directly where the user is working.

### 4. Density is part of the value

TagNote is for people who collect many notes and many tags. Product screens
should expose context: cards, counts, chips, timestamps, priority, save status,
and active filters.

Whitespace is useful when it improves scanning, touchability, or comprehension.
It is harmful when it hides the collection. Avoid product layouts that feel like
marketing pages, single-item walkthroughs, or oversized empty states.

### 5. Be calm until something matters

Most of the interface should recede. Use neutral surfaces, restrained borders,
compact controls, and terse copy. Reserve strong color and motion for states the
user must understand:

- urgent or important priority
- destructive actions
- validation errors
- failed saves
- successful save or recovery state

Never use color as the only signal. Pair it with position, border, icon, label,
or text.

### 6. Adapt by input, not by product meaning

Phone, tablet, desktop, TV, and future clients can use different containers and
gestures, but they must preserve the same product semantics.

Fixed across clients:

- Tags, notes, trash, stream, focus, and read vocabulary.
- Importance x urgency priority model.
- Background save states.
- Dense browsing posture.
- Theme families and semantic color usage.
- Import/export data shape.

Allowed to vary:

- Sidebar, tab bar, sheet, overlay, or split-view presentation.
- Hover, long-press, swipe, keyboard, or D-pad interactions.
- Native system controls for sharing, file picking, keyboard, and alerts.

### 7. Trust is product behavior

Users should never wonder whether TagNote is exploiting, trapping, or risking
their notes.

Non-negotiables:

- No dark patterns.
- No tracking in the product shell.
- Guest mode is clearly local-only.
- Export is reachable from primary chrome.
- Destructive actions have clear consequences and recovery where possible.
- Save status is honest.
- Offline read paths remain useful.

When a design creates uncertainty about data ownership, permanence, privacy, or
save state, the design is not done.

### 8. Accessibility is the baseline

Every primary workflow must work with pointer, touch, keyboard, screen reader,
and platform assistive input. Accessibility is not a later polish pass.

Design requirements:

- Visible focus on every interactive element.
- Text contrast that holds in every theme.
- Hit targets that fit the input mode.
- Screen-reader names for icon-only controls.
- Textual errors with recovery paths.
- Reduced-motion alternatives.
- Locale-aware dates, numbers, and pluralization.

If a dense layout cannot remain accessible, reduce density before reducing
accessibility.

### 9. Use the theme system, not one-off styling

Every product surface must hold up across the full theme set. Use tokens for
color, spacing, elevation, typography, and semantic state.

Do not hardcode a hex value, shadow, spacing size, or priority color because it
looks right in one theme. A design is acceptable only when it works in light and
dark variants across all families.

### 10. Write like a tool, not a brand campaign

Product copy should be terse, second-person, sentence-case, and action-oriented.

Prefer:

- "New note"
- "Save note"
- "No notes match the selected tags."
- "We couldn't reach the server. Try again."

Avoid:

- vague success copy
- exclamation-heavy celebration
- abstract nouns where verbs work
- blaming the user
- feature explanations inside the primary workflow

Marketing can be warmer, but the product should sound direct and useful.

## Decision Rules

Use these when principles pull in different directions.

| Tension | Decision rule |
| --- | --- |
| Density vs accessibility | Accessibility wins. Keep target size, focus, contrast, and readable type. |
| Speed vs trust | Optimistic UI is fine for reversible actions. Irreversible actions need confirmation. Save state must stay honest. |
| Consistency vs platform convention | Product meaning stays consistent. System affordances can be native. |
| Calm UI vs priority visibility | Priority wins when the user needs to act. Use strong signals sparingly. |
| Simplicity vs power | Prefer the smallest feature that preserves the Write -> Tag -> Stream -> Prioritize loop. |
| Offline usefulness vs implementation cost | Read paths should work offline first. Offline authoring can queue, but must say what is happening. |

## Design Review Questions

Before shipping a UX change, answer these:

- Does it strengthen Write -> Tag -> Stream -> Prioritize?
- Did it avoid adding a new organizing model?
- Can a user with many notes still scan the screen quickly?
- Are active filters, save state, priority, and destructive consequences visible?
- Does the interaction work across pointer, touch, keyboard, and screen reader?
- Does it survive all themes without hardcoded colors or spacing?
- Does the copy say what happened and what the user can do next?
- Would the same feature still make sense on phone, desktop, and a future native client?

If any answer is weak, revise the design or document the deliberate exception in
`ux_guidelines.md`.
