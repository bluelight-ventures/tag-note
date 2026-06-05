package service

import (
	"context"
	"testing"
	"time"

	"github.com/runminglu/tag-note/internal/model"
	"github.com/runminglu/tag-note/internal/repo"
)

// newServiceForTest returns a Service backed by a fresh temp DB and the id of a
// seeded user (notes have a foreign key to users).
func newServiceForTest(t *testing.T) (*Service, string) {
	t.Helper()
	r, err := repo.NewSQLiteRepo(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	const userID = "user-test"
	if err := r.CreateUser(context.Background(), userID, "u@example.com", "hash", "U", time.Now().UTC()); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return New(r), userID
}

func TestNormalizeTagsDropsReservedDefault(t *testing.T) {
	got := normalizeTags([]string{" Work ", "$default", "$DEFAULT", "work", "ideas"})
	want := []string{"work", "ideas"}
	if len(got) != len(want) {
		t.Fatalf("normalizeTags = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("normalizeTags = %v, want %v", got, want)
		}
	}
}

func TestCreateNoteAllowsEmptyContentAndTagsAndStripsReserved(t *testing.T) {
	svc, userID := newServiceForTest(t)
	ctx := context.Background()

	// Content but no tag — allowed, stored with no tags.
	if _, err := svc.CreateNote(ctx, userID, model.CreateRequest{Content: "hello", Tags: nil}); err != nil {
		t.Fatalf("CreateNote(content, no tags): %v", err)
	}
	// Fully empty note — allowed (explicit save).
	if _, err := svc.CreateNote(ctx, userID, model.CreateRequest{Content: "", Tags: nil}); err != nil {
		t.Fatalf("CreateNote(empty): %v", err)
	}
	// A user-supplied $default is stripped; the real tag is kept.
	if _, err := svc.CreateNote(ctx, userID, model.CreateRequest{Content: "tagged", Tags: []string{"$default", "Work"}}); err != nil {
		t.Fatalf("CreateNote(reserved): %v", err)
	}

	notes, err := svc.ReadNotes(ctx, userID, nil, "", 50, 0, "newest")
	if err != nil {
		t.Fatalf("ReadNotes: %v", err)
	}
	if len(notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(notes))
	}
	for _, n := range notes {
		for _, tag := range n.Tags {
			if tag == reservedDefaultTag {
				t.Fatalf("reserved %q was stored on note %q", reservedDefaultTag, n.Content)
			}
		}
		switch n.Content {
		case "hello":
			if len(n.Tags) != 0 {
				t.Fatalf("'hello' should have no tags, got %v", n.Tags)
			}
		case "tagged":
			if len(n.Tags) != 1 || n.Tags[0] != "work" {
				t.Fatalf("'tagged' should have [work], got %v", n.Tags)
			}
		}
	}
}

func TestUpdateNoteCanClearTags(t *testing.T) {
	svc, userID := newServiceForTest(t)
	ctx := context.Background()

	created, err := svc.CreateNote(ctx, userID, model.CreateRequest{Content: "x", Tags: []string{"work"}})
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	empty := []string{}
	updated, err := svc.UpdateNote(ctx, userID, created.ShortID, model.UpdateRequest{Tags: &empty})
	if err != nil {
		t.Fatalf("UpdateNote(clear tags): %v", err)
	}
	if len(updated.Tags) != 0 {
		t.Fatalf("expected tags cleared, got %v", updated.Tags)
	}
}
