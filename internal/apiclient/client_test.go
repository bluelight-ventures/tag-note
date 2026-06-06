package apiclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListNotesBuildsAuthenticatedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/notes" {
			t.Fatalf("path = %s, want /api/v1/notes", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Fatalf("User-Agent = %q", got)
		}
		q := r.URL.Query()
		if got := q["tag"]; len(got) != 2 || got[0] != "work" || got[1] != "ai" {
			t.Fatalf("tag query = %#v", got)
		}
		if got := q.Get("q"); got != "sqlite" {
			t.Fatalf("q = %q, want sqlite", got)
		}
		if got := q.Get("limit"); got != "7" {
			t.Fatalf("limit = %q, want 7", got)
		}
		if got := q.Get("offset"); got != "2" {
			t.Fatalf("offset = %q, want 2", got)
		}
		if got := q.Get("sort"); got != "updated_desc" {
			t.Fatalf("sort = %q, want updated_desc", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":"01","short_id":"01","content":"hello","created_at":"2026-01-02T03:04:05Z","tags":["work"],"pinned":true}]`))
	}))
	defer server.Close()

	client := New(server.URL, "test-token")
	client.UserAgent = "test-agent"

	notes, err := client.ListNotes(context.Background(), []string{"work", "ai"}, "sqlite", 7, 2, "updated_desc")
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("len(notes) = %d, want 1", len(notes))
	}
	if notes[0].ID != "01" || notes[0].Content != "hello" || !notes[0].Pinned {
		t.Fatalf("note = %#v", notes[0])
	}
}

func TestClientReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"ambiguous short id"}`))
	}))
	defer server.Close()

	client := New(server.URL, "test-token")
	_, err := client.GetNote(context.Background(), "01")
	if err == nil {
		t.Fatal("GetNote() error = nil")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Message != "ambiguous short id" {
		t.Fatalf("Message = %q, want ambiguous short id", apiErr.Message)
	}
}
