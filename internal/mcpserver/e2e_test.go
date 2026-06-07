package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"

	"github.com/runminglu/tag-note/internal/mcpoauth"
	"github.com/runminglu/tag-note/internal/repo"
	"github.com/runminglu/tag-note/internal/service"
)

func TestMCPE2ECreateSearchUpdateAndReadResource(t *testing.T) {
	ctx := context.Background()
	clientSession, closeSessions := connectHTTPMCPForTest(t, ctx, Config{
		DBPath:          "unused",
		PublicURL:       "http://localhost:3779",
		ResourcePath:    "/mcp",
		MaxNotes:        10,
		MaxContentBytes: 10000,
	}, "test-user", []string{mcpoauth.ScopeRead, mcpoauth.ScopeWrite, mcpoauth.ScopeDelete})
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
			"content": "Created through HTTP MCP",
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
	if search.Notes[0].Content != "Created through HTTP MCP" {
		t.Fatalf("search note content = %q", search.Notes[0].Content)
	}

	updatedContent := "Updated through HTTP MCP"
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
	ctx := context.Background()
	clientSession, closeSessions := connectHTTPMCPForTest(t, ctx, Config{
		DBPath:          "unused",
		PublicURL:       "http://localhost:3779",
		ResourcePath:    "/mcp",
		ReadOnly:        true,
		MaxNotes:        10,
		MaxContentBytes: 10000,
	}, "test-user", []string{mcpoauth.ScopeRead})
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

func TestMCPE2ERequiresBearerToken(t *testing.T) {
	ctx := context.Background()
	_, closeServer, endpoint := startHTTPMCPForTest(t, ctx, Config{
		DBPath:          "unused",
		PublicURL:       "http://localhost:3779",
		ResourcePath:    "/mcp",
		MaxNotes:        10,
		MaxContentBytes: 10000,
	}, "test-user", []string{mcpoauth.ScopeRead})
	defer closeServer()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 401: %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("WWW-Authenticate"); !strings.Contains(got, `resource_metadata="http://localhost:3779/.well-known/oauth-protected-resource/mcp"`) {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func connectHTTPMCPForTest(t *testing.T, ctx context.Context, cfg Config, userID string, scopes []string) (*mcp.ClientSession, func()) {
	t.Helper()
	_, closeServer, endpoint := startHTTPMCPForTest(t, ctx, cfg, userID, scopes)
	transport := &mcp.StreamableClientTransport{
		Endpoint: endpoint,
		OAuthHandler: staticOAuthHandler{
			token: &oauth2.Token{AccessToken: "test-access-token", TokenType: "Bearer"},
		},
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "tagnote-mcp-e2e", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, transport, nil)
	if err != nil {
		closeServer()
		t.Fatalf("client Connect() error = %v", err)
	}
	return clientSession, func() {
		clientSession.Close()
		closeServer()
	}
}

func startHTTPMCPForTest(t *testing.T, ctx context.Context, cfg Config, userID string, scopes []string) (*httptest.Server, func(), string) {
	t.Helper()
	store, err := repo.NewSQLiteRepo(t.TempDir() + "/tagnote.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo() error = %v", err)
	}
	if err := store.CreateUser(ctx, userID, "mcp-e2e@example.com", "hash", "MCP E2E", time.Now().UTC()); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	server, err := New(cfg, service.New(store), "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	streamHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)
	verifier := func(_ context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
		if token != "test-access-token" {
			return nil, sdkauth.ErrInvalidToken
		}
		return &sdkauth.TokenInfo{
			Scopes:     scopes,
			UserID:     userID,
			Expiration: time.Now().Add(time.Hour),
		}, nil
	}
	mux := http.NewServeMux()
	mux.Handle(cfg.ResourcePath, sdkauth.RequireBearerToken(verifier, &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: cfg.ResourceMetadataURL(),
	})(streamHandler))
	httpServer := httptest.NewServer(mux)
	return httpServer, func() {
		httpServer.Close()
		store.Close()
	}, httpServer.URL + cfg.ResourcePath
}

type staticOAuthHandler struct {
	token *oauth2.Token
}

func (h staticOAuthHandler) TokenSource(context.Context) (oauth2.TokenSource, error) {
	return oauth2.StaticTokenSource(h.token), nil
}

func (h staticOAuthHandler) Authorize(_ context.Context, _ *http.Request, resp *http.Response) error {
	resp.Body.Close()
	return nil
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
