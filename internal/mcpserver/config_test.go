package mcpserver

import (
	"testing"
	"time"

	"github.com/runminglu/tag-note/internal/model"
)

func TestConfigValidateRequiresToken(t *testing.T) {
	cfg := Config{BaseURL: "http://localhost:3000"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil")
	}
}

func TestConfigValidateDefaultsCaps(t *testing.T) {
	cfg := Config{Token: "token", MaxNotes: -1, MaxContentBytes: -1}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.MaxNotes != defaultMaxNotes {
		t.Fatalf("MaxNotes = %d, want %d", cfg.MaxNotes, defaultMaxNotes)
	}
	if cfg.MaxContentBytes != defaultMaxContentBytes {
		t.Fatalf("MaxContentBytes = %d, want %d", cfg.MaxContentBytes, defaultMaxContentBytes)
	}
}

func TestNoteViewsCapContent(t *testing.T) {
	notes := []model.SubNote{{
		ID:        "01",
		ShortID:   "01",
		Content:   "abcdef",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Tags:      []string{"work"},
	}}
	views, truncated := noteViews(notes, true, 3)
	if !truncated {
		t.Fatal("truncated = false, want true")
	}
	if len(views) != 1 {
		t.Fatalf("len(views) = %d, want 1", len(views))
	}
	if views[0].Content != "abc" {
		t.Fatalf("Content = %q, want abc", views[0].Content)
	}
}
