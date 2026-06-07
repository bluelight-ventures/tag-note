package mcpserver

import (
	"time"

	"github.com/runminglu/tag-note/internal/model"
)

// NoteView is the MCP-facing note shape. Content is optional for search output.
type NoteView struct {
	ID        string     `json:"id"`
	ShortID   string     `json:"short_id"`
	Content   string     `json:"content,omitempty"`
	Snippet   string     `json:"snippet,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Tags      []string   `json:"tags"`
	Pinned    bool       `json:"pinned"`
}

func noteView(note model.SubNote, includeContent bool) NoteView {
	view := NoteView{
		ID:        note.ID,
		ShortID:   note.ShortID,
		Snippet:   note.Snippet,
		CreatedAt: note.CreatedAt,
		UpdatedAt: note.UpdatedAt,
		Tags:      note.Tags,
		Pinned:    note.Pinned,
	}
	if includeContent {
		view.Content = note.Content
	}
	return view
}

func noteViews(notes []model.SubNote, includeContent bool, maxContentBytes int) ([]NoteView, bool) {
	views := make([]NoteView, 0, len(notes))
	remaining := maxContentBytes
	truncated := false
	for _, note := range notes {
		view := noteView(note, includeContent)
		if includeContent {
			content, cut := capString(view.Content, remaining)
			view.Content = content
			truncated = truncated || cut
			remaining -= len(content)
			if remaining < 0 {
				remaining = 0
			}
		}
		views = append(views, view)
	}
	return views, truncated
}

func capString(s string, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		if s == "" {
			return "", false
		}
		return "", true
	}
	if len(s) <= maxBytes {
		return s, false
	}
	return s[:maxBytes], true
}

func (s *Server) cappedLimit(requested int) int {
	if requested <= 0 || requested > s.cfg.MaxNotes {
		return s.cfg.MaxNotes
	}
	return requested
}

func (s *Server) capNotes(notes []model.SubNote) []model.SubNote {
	if len(notes) > s.cfg.MaxNotes {
		return notes[:s.cfg.MaxNotes]
	}
	return notes
}
