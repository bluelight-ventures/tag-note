package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/runminglu/tag-note/internal/model"
)

func TestMCPE2ECreateSearchUpdateAndReadResource(t *testing.T) {
	api := newFakeTagNoteAPI(t)
	defer api.server.Close()

	ctx := context.Background()
	clientSession, closeSessions := connectMCPForTest(t, ctx, Config{
		BaseURL:         api.server.URL,
		Token:           "test-token",
		MaxNotes:        10,
		MaxContentBytes: 10000,
	}, "test")
	defer closeSessions()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if !hasTool(tools.Tools, "tagnote_create_note") {
		t.Fatalf("tagnote_create_note tool not registered: %#v", toolNames(tools.Tools))
	}

	createResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "tagnote_create_note",
		Arguments: map[string]any{
			"content": "Created through MCP",
			"tags":    []string{"mcp", "e2e"},
			"pinned":  true,
		},
	})
	if err != nil {
		t.Fatalf("create CallTool() error = %v", err)
	}
	assertToolOK(t, createResult)
	created := decodeStructured[noteOutput](t, createResult)
	if created.Note.ID == "" {
		t.Fatal("created note ID is empty")
	}
	if !created.Note.Pinned {
		t.Fatal("created note is not pinned")
	}

	searchResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "tagnote_search_notes",
		Arguments: map[string]any{
			"tags":            []string{"mcp"},
			"query":           "Created",
			"include_content": true,
		},
	})
	if err != nil {
		t.Fatalf("search CallTool() error = %v", err)
	}
	assertToolOK(t, searchResult)
	search := decodeStructured[notesOutput](t, searchResult)
	if search.Count != 1 {
		t.Fatalf("search count = %d, want 1", search.Count)
	}
	if search.Notes[0].Content != "Created through MCP" {
		t.Fatalf("search note content = %q", search.Notes[0].Content)
	}

	updatedContent := "Updated through MCP"
	updateResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "tagnote_update_note",
		Arguments: map[string]any{
			"id":      created.Note.ID,
			"content": updatedContent,
			"tags":    []string{"mcp", "updated"},
		},
	})
	if err != nil {
		t.Fatalf("update CallTool() error = %v", err)
	}
	assertToolOK(t, updateResult)
	updated := decodeStructured[noteOutput](t, updateResult)
	if updated.Note.Content != updatedContent {
		t.Fatalf("updated content = %q", updated.Note.Content)
	}
	if strings.Join(updated.Note.Tags, ",") != "mcp,updated" {
		t.Fatalf("updated tags = %#v", updated.Note.Tags)
	}

	pinResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "tagnote_set_note_pinned",
		Arguments: map[string]any{
			"id":     created.Note.ID,
			"pinned": false,
		},
	})
	if err != nil {
		t.Fatalf("pin CallTool() error = %v", err)
	}
	assertToolOK(t, pinResult)
	pinned := decodeStructured[noteOutput](t, pinResult)
	if pinned.Note.Pinned {
		t.Fatal("note is still pinned")
	}

	resource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: "tagnote://notes/" + created.Note.ID + ".md",
	})
	if err != nil {
		t.Fatalf("ReadResource() error = %v", err)
	}
	if len(resource.Contents) != 1 {
		t.Fatalf("resource contents = %d, want 1", len(resource.Contents))
	}
	if resource.Contents[0].Text != updatedContent {
		t.Fatalf("resource text = %q", resource.Contents[0].Text)
	}
}

func TestMCPE2EReadOnlyModeHidesWriteTools(t *testing.T) {
	api := newFakeTagNoteAPI(t)
	defer api.server.Close()

	ctx := context.Background()
	clientSession, closeSessions := connectMCPForTest(t, ctx, Config{
		BaseURL:         api.server.URL,
		Token:           "test-token",
		ReadOnly:        true,
		MaxNotes:        10,
		MaxContentBytes: 10000,
	}, "test")
	defer closeSessions()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if hasTool(tools.Tools, "tagnote_create_note") {
		t.Fatalf("read-only mode registered write tool: %#v", toolNames(tools.Tools))
	}
	if !hasTool(tools.Tools, "tagnote_search_notes") {
		t.Fatalf("read-only mode did not register read tool: %#v", toolNames(tools.Tools))
	}
}

func connectMCPForTest(t *testing.T, ctx context.Context, cfg Config, version string) (*mcp.ClientSession, func()) {
	t.Helper()
	server, err := New(cfg, version)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect() error = %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "tagnote-mcp-e2e", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		serverSession.Close()
		t.Fatalf("client Connect() error = %v", err)
	}
	return clientSession, func() {
		clientSession.Close()
		serverSession.Close()
	}
}

func assertToolOK(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	if result == nil {
		t.Fatal("tool result is nil")
	}
	if result.IsError {
		var msg string
		if len(result.Content) > 0 {
			if text, ok := result.Content[0].(*mcp.TextContent); ok {
				msg = text.Text
			}
		}
		t.Fatalf("tool returned error: %s", msg)
	}
}

func decodeStructured[T any](t *testing.T, result *mcp.CallToolResult) T {
	t.Helper()
	var out T
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v; json=%s", err, string(b))
	}
	return out
}

func hasTool(tools []*mcp.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func toolNames(tools []*mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

type fakeTagNoteAPI struct {
	t      *testing.T
	server *httptest.Server
	mu     sync.Mutex
	nextID int
	notes  map[string]model.SubNote
}

func newFakeTagNoteAPI(t *testing.T) *fakeTagNoteAPI {
	api := &fakeTagNoteAPI{
		t:      t,
		nextID: 1,
		notes:  make(map[string]model.SubNote),
	}
	api.server = httptest.NewServer(http.HandlerFunc(api.serveHTTP))
	return api
}

func (a *fakeTagNoteAPI) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
		http.Error(w, `{"error":"missing or invalid token"}`, http.StatusUnauthorized)
		return
	}
	if r.URL.Path == "/api/v1/notes" {
		switch r.Method {
		case http.MethodPost:
			a.createNote(w, r)
			return
		case http.MethodGet:
			a.listNotes(w, r)
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/notes/") {
		a.noteByID(w, r)
		return
	}
	http.NotFound(w, r)
}

func (a *fakeTagNoteAPI) createNote(w http.ResponseWriter, r *http.Request) {
	var req model.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	id := fmt.Sprintf("01MCP%021d", a.nextID)
	a.nextID++
	createdAt := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	a.notes[id] = model.SubNote{
		ID:        id,
		ShortID:   model.MakeShortID(id),
		Content:   req.Content,
		CreatedAt: createdAt,
		Tags:      append([]string(nil), req.Tags...),
	}
	writeJSON(w, http.StatusCreated, model.CreateResponse{ID: id, ShortID: model.MakeShortID(id), CreatedAt: createdAt})
}

func (a *fakeTagNoteAPI) listNotes(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var notes []model.SubNote
	for _, note := range a.notes {
		if matchesTags(note.Tags, r.URL.Query()["tag"]) && matchesQuery(note.Content, r.URL.Query().Get("q")) {
			notes = append(notes, note)
		}
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].ID < notes[j].ID })
	writeJSON(w, http.StatusOK, notes)
}

func (a *fakeTagNoteAPI) noteByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/notes/")
	if strings.HasSuffix(path, "/pin") {
		id := strings.TrimSuffix(path, "/pin")
		if r.Method != http.MethodPut {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		a.togglePin(w, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.getNote(w, path)
	case http.MethodPut:
		a.updateNote(w, r, path)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (a *fakeTagNoteAPI) getNote(w http.ResponseWriter, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	note, ok := a.notes[id]
	if !ok {
		http.Error(w, `{"error":"note not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (a *fakeTagNoteAPI) updateNote(w http.ResponseWriter, r *http.Request, id string) {
	var req model.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	note, ok := a.notes[id]
	if !ok {
		http.Error(w, `{"error":"note not found"}`, http.StatusNotFound)
		return
	}
	if req.Content != nil {
		note.Content = *req.Content
		now := time.Date(2026, 6, 7, 12, 30, 0, 0, time.UTC)
		note.UpdatedAt = &now
	}
	if req.Tags != nil {
		note.Tags = append([]string(nil), (*req.Tags)...)
	}
	a.notes[id] = note
	writeJSON(w, http.StatusOK, note)
}

func (a *fakeTagNoteAPI) togglePin(w http.ResponseWriter, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	note, ok := a.notes[id]
	if !ok {
		http.Error(w, `{"error":"note not found"}`, http.StatusNotFound)
		return
	}
	note.Pinned = !note.Pinned
	a.notes[id] = note
	w.WriteHeader(http.StatusNoContent)
}

func matchesTags(noteTags, required []string) bool {
	for _, req := range required {
		found := false
		for _, tag := range noteTags {
			if tag == req {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func matchesQuery(content, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(content), strings.ToLower(query))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
