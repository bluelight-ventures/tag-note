package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/runminglu/tag-note/internal/repo"
)

func TestGoogleTokenInfoIsEmailVerified(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{name: "bool true", value: true, want: true},
		{name: "bool false", value: false, want: false},
		{name: "string true", value: "true", want: true},
		{name: "string true uppercase", value: "TRUE", want: true},
		{name: "string false", value: "false", want: false},
		{name: "missing", value: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := googleTokenInfo{EmailVerified: tt.value}
			if got := info.isEmailVerified(); got != tt.want {
				t.Fatalf("isEmailVerified() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitClientIDsAndMembership(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{raw: "", want: nil},
		{raw: "web-id", want: []string{"web-id"}},
		{raw: "web-id,ios-id", want: []string{"web-id", "ios-id"}},
		{raw: " web-id , ios-id , ", want: []string{"web-id", "ios-id"}},
	}
	for _, tc := range cases {
		got := splitClientIDs(tc.raw)
		if len(got) != len(tc.want) {
			t.Fatalf("splitClientIDs(%q) = %v, want %v", tc.raw, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("splitClientIDs(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		}
	}

	ids := splitClientIDs("web-id,ios-id")
	if !containsString(ids, "ios-id") || !containsString(ids, "web-id") {
		t.Fatalf("containsString should match both configured audiences")
	}
	if containsString(ids, "attacker-id") {
		t.Fatalf("containsString should reject an unknown audience")
	}
}

func TestDeleteAccountRemovesUploadFiles(t *testing.T) {
	t.Setenv("TAGNOTE_ALLOW_DEV_SECRET", "1")

	ctx := context.Background()
	r, err := repo.NewSQLiteRepo(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo() error = %v", err)
	}
	defer r.Close()

	uploadDir := t.TempDir()
	auth, err := NewAuth(r, NewEmailService(), uploadDir)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}

	now := time.Now().UTC()
	userID := "user-delete"
	otherID := "user-keep"
	if err := r.CreateUser(ctx, userID, "delete@example.com", "hash", "Delete Me", now); err != nil {
		t.Fatalf("CreateUser(delete) error = %v", err)
	}
	if err := r.CreateUser(ctx, otherID, "keep@example.com", "hash", "Keep Me", now); err != nil {
		t.Fatalf("CreateUser(keep) error = %v", err)
	}

	for _, filename := range []string{"owned.png", "legacy.jpg", "other.png"} {
		if err := os.WriteFile(filepath.Join(uploadDir, filename), []byte("image"), 0600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", filename, err)
		}
	}
	if err := r.CreateUpload(ctx, userID, "upload-owned", "owned.png", "image/png", 10, now); err != nil {
		t.Fatalf("CreateUpload(owned) error = %v", err)
	}
	if err := r.CreateUpload(ctx, otherID, "upload-other", "other.png", "image/png", 10, now); err != nil {
		t.Fatalf("CreateUpload(other) error = %v", err)
	}
	content := "![legacy](/uploads/legacy.jpg) ![other](/uploads/other.png)"
	if err := r.Create(ctx, userID, "note-delete", content, []string{"delete-tag"}, now); err != nil {
		t.Fatalf("Create(delete note) error = %v", err)
	}

	if err := auth.DeleteAccount(ctx, userID); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}

	for _, filename := range []string{"owned.png", "legacy.jpg"} {
		if _, err := os.Stat(filepath.Join(uploadDir, filename)); !os.IsNotExist(err) {
			t.Fatalf("Stat(%s) error = %v, want not exist", filename, err)
		}
	}
	if _, err := os.Stat(filepath.Join(uploadDir, "other.png")); err != nil {
		t.Fatalf("Stat(other.png) error = %v, want file to remain", err)
	}
	if _, err := r.FindUserByID(ctx, userID); err != repo.ErrNotFound {
		t.Fatalf("FindUserByID(deleted) error = %v, want ErrNotFound", err)
	}
}
