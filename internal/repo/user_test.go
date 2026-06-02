package repo

import (
	"context"
	"testing"
	"time"

	"github.com/runminglu/tag-note/internal/model"
)

func TestDeleteUserRemovesAccountData(t *testing.T) {
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
	if err := r.Create(ctx, userID, "note-delete", "delete content", []string{"delete-tag"}, now); err != nil {
		t.Fatalf("Create(delete note) error = %v", err)
	}
	if err := r.Create(ctx, otherID, "note-keep", "keep content", []string{"keep-tag"}, now); err != nil {
		t.Fatalf("Create(keep note) error = %v", err)
	}
	if err := r.SaveSettings(ctx, userID, model.Settings{Theme: "nord-dark"}); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}
	if err := r.CreatePasswordResetToken(ctx, "reset-delete", userID, "reset-token", now.Add(time.Hour)); err != nil {
		t.Fatalf("CreatePasswordResetToken() error = %v", err)
	}
	if err := r.CreateUpload(ctx, userID, "upload-delete", "delete.png", "image/png", 123, now); err != nil {
		t.Fatalf("CreateUpload() error = %v", err)
	}

	if err := r.DeleteUser(ctx, userID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	if _, err := r.FindUserByID(ctx, userID); err != ErrNotFound {
		t.Fatalf("FindUserByID(deleted) error = %v, want ErrNotFound", err)
	}
	if _, err := r.FindByID(ctx, userID, "note-delete"); err != ErrNotFound {
		t.Fatalf("FindByID(deleted note) error = %v, want ErrNotFound", err)
	}
	notes, err := r.Search(ctx, otherID, nil, "", 0, 0, "")
	if err != nil {
		t.Fatalf("Search(other) error = %v", err)
	}
	if len(notes) != 1 || notes[0].ID != "note-keep" {
		t.Fatalf("Search(other) = %#v, want note-keep", notes)
	}
	var uploadCount int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM uploads WHERE user_id = ?`, userID).Scan(&uploadCount); err != nil {
		t.Fatalf("count uploads: %v", err)
	}
	if uploadCount != 0 {
		t.Fatalf("deleted user upload count = %d, want 0", uploadCount)
	}
}
