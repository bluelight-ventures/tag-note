package repo

import (
	"context"
	"testing"
	"time"
)

func TestListUserUploadFilenamesIncludesOwnedAndLegacyReferences(t *testing.T) {
	ctx := context.Background()
	r, err := NewSQLiteRepo(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo() error = %v", err)
	}
	defer r.Close()

	now := time.Now().UTC()
	userID := "user-delete"
	otherID := "user-keep"
	if err := r.CreateUser(ctx, userID, "delete@example.com", "hash", "Delete Me", now); err != nil {
		t.Fatalf("CreateUser(delete) error = %v", err)
	}
	if err := r.CreateUser(ctx, otherID, "keep@example.com", "hash", "Keep Me", now); err != nil {
		t.Fatalf("CreateUser(keep) error = %v", err)
	}
	if err := r.CreateUpload(ctx, userID, "upload-owned", "owned.png", "image/png", 10, now); err != nil {
		t.Fatalf("CreateUpload(owned) error = %v", err)
	}
	if err := r.CreateUpload(ctx, otherID, "upload-other", "other.png", "image/png", 10, now); err != nil {
		t.Fatalf("CreateUpload(other) error = %v", err)
	}

	content := "![legacy](/uploads/legacy.jpg) ![owned](/uploads/owned.png?size=large) ![other](/uploads/other.png)"
	if err := r.Create(ctx, userID, "note-delete", content, []string{"tag"}, now); err != nil {
		t.Fatalf("Create(delete note) error = %v", err)
	}

	got, err := r.ListUserUploadFilenames(ctx, userID)
	if err != nil {
		t.Fatalf("ListUserUploadFilenames() error = %v", err)
	}
	gotSet := map[string]bool{}
	for _, filename := range got {
		gotSet[filename] = true
	}
	for _, want := range []string{"owned.png", "legacy.jpg"} {
		if !gotSet[want] {
			t.Fatalf("ListUserUploadFilenames() = %v, missing %q", got, want)
		}
	}
	if gotSet["other.png"] {
		t.Fatalf("ListUserUploadFilenames() = %v, included upload owned by another user", got)
	}
}

func TestListUserUploadFilenamesSkipsLegacyReferencedByOtherUser(t *testing.T) {
	ctx := context.Background()
	r, err := NewSQLiteRepo(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo() error = %v", err)
	}
	defer r.Close()

	now := time.Now().UTC()
	userID := "user-delete"
	otherID := "user-keep"
	if err := r.CreateUser(ctx, userID, "delete@example.com", "hash", "Delete Me", now); err != nil {
		t.Fatalf("CreateUser(delete) error = %v", err)
	}
	if err := r.CreateUser(ctx, otherID, "keep@example.com", "hash", "Keep Me", now); err != nil {
		t.Fatalf("CreateUser(keep) error = %v", err)
	}

	// A legacy (untracked) upload referenced by both users, plus one referenced
	// only by the deleting user.
	if err := r.Create(ctx, userID, "note-delete", "![shared](/uploads/shared.jpg) ![solo](/uploads/solo.jpg)", []string{"tag"}, now); err != nil {
		t.Fatalf("Create(delete note) error = %v", err)
	}
	if err := r.Create(ctx, otherID, "note-keep", "![shared](/uploads/shared.jpg)", []string{"tag"}, now); err != nil {
		t.Fatalf("Create(keep note) error = %v", err)
	}

	got, err := r.ListUserUploadFilenames(ctx, userID)
	if err != nil {
		t.Fatalf("ListUserUploadFilenames() error = %v", err)
	}
	gotSet := map[string]bool{}
	for _, filename := range got {
		gotSet[filename] = true
	}
	if !gotSet["solo.jpg"] {
		t.Fatalf("ListUserUploadFilenames() = %v, missing solo.jpg", got)
	}
	if gotSet["shared.jpg"] {
		t.Fatalf("ListUserUploadFilenames() = %v, included file another user references", got)
	}
}

func TestExtractUploadFilenames(t *testing.T) {
	content := `![a](/uploads/a.png) <img src="/uploads/b.jpg?x=1"> https://tag-note.com/uploads/c.webp#frag /uploads/nested/nope.png`
	got := extractUploadFilenames(content)
	gotSet := map[string]bool{}
	for _, filename := range got {
		gotSet[filename] = true
	}
	for _, want := range []string{"a.png", "b.jpg", "c.webp"} {
		if !gotSet[want] {
			t.Fatalf("extractUploadFilenames() = %v, missing %q", got, want)
		}
	}
	if gotSet["nope.png"] {
		t.Fatalf("extractUploadFilenames() = %v, included nested upload path", got)
	}
}
