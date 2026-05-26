package service

import (
	"context"
	"crypto/rand"
	"log"
	"time"

	"github.com/oklog/ulid/v2"
)

// seedOnboardingContent creates welcome notes and example tags for a new user.
// Errors are logged but do not fail registration.
func (a *AuthService) seedOnboardingContent(ctx context.Context, userID string) {
	now := time.Now().UTC()

	type seedNote struct {
		content string
		tags    []string
		pin     bool
	}

	notes := []seedNote{
		{content: seedNoteMarkdown, tags: []string{"tips", "markdown"}, pin: false},
		{content: seedNotePriority, tags: []string{"tips", "priority"}, pin: false},
		{content: seedNoteTags, tags: []string{"tips", "tags"}, pin: false},
		{content: seedNoteWelcome, tags: []string{"welcome", "getting-started"}, pin: true},
	}

	for i, n := range notes {
		// Stagger timestamps so notes appear in correct order (welcome last = newest first)
		ts := now.Add(time.Duration(i) * time.Second)
		id := ulid.MustNew(ulid.Timestamp(ts), rand.Reader).String()

		if err := a.repo.Create(ctx, userID, id, n.content, n.tags, ts); err != nil {
			log.Printf("warning: seed note %d for user %s: %v", i, userID, err)
			return
		}

		if n.pin {
			if err := a.repo.TogglePin(ctx, userID, id); err != nil {
				log.Printf("warning: pin seed note for user %s: %v", userID, err)
			}
		}
	}

	// Approve all tags so they don't appear as "unreviewed"
	if err := a.repo.ApproveAllTags(ctx, userID); err != nil {
		log.Printf("warning: approve seed tags for user %s: %v", userID, err)
	}

	// Set priority on welcome/getting-started tags
	priorities := map[string][2]int{
		"welcome":         {80, 80},
		"getting-started": {80, 80},
		"tips":            {60, 30},
	}
	for tag, p := range priorities {
		if err := a.repo.UpdateTagPriority(ctx, userID, tag, p[0], p[1]); err != nil {
			log.Printf("warning: set priority for seed tag %q user %s: %v", tag, userID, err)
		}
	}
}

const seedNoteWelcome = `# Welcome to TagNote!

TagNote organizes your notes with **tags** instead of folders. Here's what makes it different:

- **Tag freely** — every note can have multiple tags, so nothing gets lost in a single folder
- **Filter by tags** — click tags in the sidebar to see only matching notes
- **Combine tags** — filter by multiple tags at once to zoom in on exactly what you need
- **Search everything** — full-text search works alongside tag filters

## Quick start

1. Click **New note** in the sidebar to create your first note
2. Write in Markdown (this editor supports bold, lists, headings, images, and more)
3. Add tags in the tag field above the editor — type and press Enter
4. Click any tag in the sidebar to filter your notes

Take a look at the other example notes to see tags and priorities in action. When you're ready, feel free to delete these notes and start fresh!`

const seedNoteTags = `# How tags work

Every note in TagNote gets one or more tags. Unlike folders, a note can belong to many categories at once.

## Filtering

- Click a tag in the **sidebar tag cloud** to filter notes
- Click additional tags to narrow down further (AND logic)
- Click an active tag again to remove the filter
- Use the **search bar** to search within your filtered results

## Managing tags

Open the **Tags** tab in the sidebar to:

- Approve or rename tags
- Set importance and urgency (see the priority note)
- Delete tags you no longer need

Tags are created automatically when you add them to a note — no setup needed.`

const seedNotePriority = `# The priority system

TagNote uses an **Eisenhower-style** priority system based on two axes:

- **Importance** (0–100): How much does this matter?
- **Urgency** (0–100): How soon does it need attention?

## How to set priorities

1. Go to the **Tags** tab in the sidebar
2. Click on a tag to expand its settings
3. Adjust the importance and urgency sliders

## Color coding

Notes are color-coded based on the highest-priority tag they carry:

- **Red border** — high importance + high urgency (do first)
- **Amber border** — high on one axis (plan or delegate)
- **No border** — low priority (do later or drop)

Try adjusting the priority on the ` + "`tips`" + ` tag in the Tags tab to see the colors change on these example notes.`

const seedNoteMarkdown = `# Writing with Markdown

TagNote uses a rich Markdown editor. Here are some things you can do:

## Formatting

- **Bold** with ` + "`**text**`" + `
- *Italic* with ` + "`*text*`" + `
- ` + "`Code`" + ` with backticks
- [Links](https://example.com) with ` + "`[text](url)`" + `

## Lists and structure

1. Numbered lists
2. Like this one

- Bullet lists
- Like this one

> Blockquotes for callouts

## Images

Paste an image from your clipboard, drag and drop a file, or use the toolbar button. Images are uploaded and stored alongside your notes.

## Themes

Click the theme button (sun icon) in the sidebar to cycle through five themes: Light, Dark, Nord, Solarized, and Rosé Pine.`
