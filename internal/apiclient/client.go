package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/runminglu/tag-note/internal/model"
)

const (
	defaultBaseURL   = "http://localhost:3000"
	defaultTimeout   = 10 * time.Second
	defaultUserAgent = "tagnote-cli"
)

// Error is returned when the TagNote API responds with a non-2xx status.
type Error struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("server error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("server error (%d): %s", e.StatusCode, e.Body)
}

// Client is an authenticated HTTP client for the TagNote API.
type Client struct {
	BaseURL    string
	Token      string
	UserAgent  string
	HTTPClient *http.Client
}

// New creates a TagNote API client.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Token:     token,
		UserAgent: defaultUserAgent,
		HTTPClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Default returns a client configured from TAGNOTE_URL and TAGNOTE_TOKEN.
func Default() *Client {
	return New(BaseURL(), authToken())
}

// BaseURL returns the server URL, checking TAGNOTE_URL env var first.
func BaseURL() string {
	if u := os.Getenv("TAGNOTE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return defaultBaseURL
}

// authToken returns the JWT token from TAGNOTE_TOKEN env var.
func authToken() string {
	return os.Getenv("TAGNOTE_TOKEN")
}

func (c *Client) baseURL() string {
	if c.BaseURL == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: defaultTimeout}
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	return req, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any, wantStatus ...int) error {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()

	if !statusOK(resp.StatusCode, wantStatus...) {
		return apiError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func statusOK(status int, wantStatus ...int) bool {
	if len(wantStatus) == 0 {
		return status >= 200 && status < 300
	}
	for _, want := range wantStatus {
		if status == want {
			return true
		}
	}
	return false
}

func apiError(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	body := strings.TrimSpace(string(b))
	var payload struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(b, &payload)
	msg := strings.TrimSpace(payload.Error)
	if msg == "" {
		msg = body
	}
	return &Error{StatusCode: resp.StatusCode, Message: msg, Body: body}
}

// Login authenticates with the server and returns a JWT token.
func (c *Client) Login(ctx context.Context, email, password string) (string, error) {
	var result struct {
		Token string `json:"token"`
	}
	err := c.do(ctx, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, &result, http.StatusOK)
	if err != nil {
		return "", fmt.Errorf("login failed: %w", err)
	}
	return result.Token, nil
}

// CreateNote creates a new sub-note.
func (c *Client) CreateNote(ctx context.Context, content string, tags []string) (*model.CreateResponse, error) {
	var result model.CreateResponse
	err := c.do(ctx, http.MethodPost, "/api/v1/notes", model.CreateRequest{
		Content: content,
		Tags:    tags,
	}, &result, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListNotes fetches notes matching tags and/or search query.
func (c *Client) ListNotes(ctx context.Context, tags []string, query string, limit, offset int, sort string) ([]model.SubNote, error) {
	path := "/api/v1/notes?" + buildQuery(tags, query, limit, offset, sort)
	var notes []model.SubNote
	if err := c.do(ctx, http.MethodGet, path, nil, &notes); err != nil {
		return nil, err
	}
	if notes == nil {
		notes = []model.SubNote{}
	}
	return notes, nil
}

// GetNote fetches a single note by full ID or unambiguous short ID.
func (c *Client) GetNote(ctx context.Context, id string) (*model.SubNote, error) {
	var note model.SubNote
	if err := c.do(ctx, http.MethodGet, "/api/v1/notes/"+url.PathEscape(id), nil, &note); err != nil {
		return nil, err
	}
	return &note, nil
}

// UpdateNote updates a note's content and/or tags.
func (c *Client) UpdateNote(ctx context.Context, id string, req model.UpdateRequest) (*model.SubNote, error) {
	var note model.SubNote
	if err := c.do(ctx, http.MethodPut, "/api/v1/notes/"+url.PathEscape(id), req, &note); err != nil {
		return nil, err
	}
	return &note, nil
}

// DeleteNote soft-deletes a note.
func (c *Client) DeleteNote(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/notes/"+url.PathEscape(id), nil, nil, http.StatusNoContent)
}

// RestoreNote restores a soft-deleted note.
func (c *Client) RestoreNote(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPut, "/api/v1/notes/"+url.PathEscape(id)+"/restore", nil, nil, http.StatusNoContent)
}

// TogglePin toggles a note's pinned state.
func (c *Client) TogglePin(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPut, "/api/v1/notes/"+url.PathEscape(id)+"/pin", nil, nil, http.StatusNoContent)
}

// RenderStream fetches the Markdown stream for the given tags and/or search query.
func (c *Client) RenderStream(ctx context.Context, tags []string, query string) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/notes/stream?"+buildQuery(tags, query, 0, 0, ""), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	if !statusOK(resp.StatusCode) {
		return "", apiError(resp)
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// ListTrashed fetches soft-deleted notes.
func (c *Client) ListTrashed(ctx context.Context) ([]model.SubNote, error) {
	var notes []model.SubNote
	if err := c.do(ctx, http.MethodGet, "/api/v1/notes/trash", nil, &notes); err != nil {
		return nil, err
	}
	if notes == nil {
		notes = []model.SubNote{}
	}
	return notes, nil
}

// ListTags fetches tag names.
func (c *Client) ListTags(ctx context.Context, limit int) ([]string, error) {
	v := url.Values{}
	if limit > 0 {
		v.Set("limit", fmt.Sprint(limit))
	}
	path := "/api/v1/tags"
	if encoded := v.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var tags []string
	if err := c.do(ctx, http.MethodGet, path, nil, &tags); err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

// ListTagsDetailed fetches all tags with status and note counts.
func (c *Client) ListTagsDetailed(ctx context.Context) ([]model.TagInfo, error) {
	var tags []model.TagInfo
	if err := c.do(ctx, http.MethodGet, "/api/v1/tags/detailed", nil, &tags); err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []model.TagInfo{}
	}
	return tags, nil
}

// AutocompleteTags fetches tag names matching a prefix.
func (c *Client) AutocompleteTags(ctx context.Context, prefix string, limit int) ([]string, error) {
	v := url.Values{}
	v.Set("q", prefix)
	if limit > 0 {
		v.Set("limit", fmt.Sprint(limit))
	}
	var tags []string
	if err := c.do(ctx, http.MethodGet, "/api/v1/tags/autocomplete?"+v.Encode(), nil, &tags); err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

// ApproveTag approves a tag.
func (c *Client) ApproveTag(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodPut, "/api/v1/tags/"+url.PathEscape(name)+"/approve", nil, nil, http.StatusNoContent)
}

// RenameTag renames or merges a tag.
func (c *Client) RenameTag(ctx context.Context, oldName, newName string) error {
	return c.do(ctx, http.MethodPut, "/api/v1/tags/"+url.PathEscape(oldName)+"/rename", model.TagRenameRequest{
		NewName: newName,
	}, nil, http.StatusNoContent)
}

// DeleteTag deletes a tag association from all notes.
func (c *Client) DeleteTag(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/tags/"+url.PathEscape(name), nil, nil, http.StatusNoContent)
}

// UpdateTagPriority updates a tag's priority coordinates.
func (c *Client) UpdateTagPriority(ctx context.Context, name string, importance, urgency int) error {
	return c.do(ctx, http.MethodPut, "/api/v1/tags/"+url.PathEscape(name)+"/priority", model.TagPriorityRequest{
		Importance: &importance,
		Urgency:    &urgency,
	}, nil, http.StatusNoContent)
}

// GetSettings fetches persisted user settings.
func (c *Client) GetSettings(ctx context.Context) (*model.Settings, error) {
	var settings model.Settings
	if err := c.do(ctx, http.MethodGet, "/api/v1/settings", nil, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// SaveSettings persists user settings.
func (c *Client) SaveSettings(ctx context.Context, settings model.Settings) (*model.Settings, error) {
	var result model.Settings
	if err := c.do(ctx, http.MethodPut, "/api/v1/settings", settings, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func buildQuery(tags []string, query string, limit, offset int, sort string) string {
	v := url.Values{}
	for _, t := range tags {
		v.Add("tag", t)
	}
	if query != "" {
		v.Set("q", query)
	}
	if limit > 0 {
		v.Set("limit", fmt.Sprint(limit))
	}
	if offset > 0 {
		v.Set("offset", fmt.Sprint(offset))
	}
	if sort != "" {
		v.Set("sort", sort)
	}
	return v.Encode()
}

// Login authenticates with the server and returns a JWT token.
func Login(email, password string) (string, error) {
	return Default().Login(context.Background(), email, password)
}

// CreateNote sends a POST to create a new sub-note.
func CreateNote(content string, tags []string) (*model.CreateResponse, error) {
	return Default().CreateNote(context.Background(), content, tags)
}

// ReadStream fetches the Markdown stream for the given tags and/or search query.
func ReadStream(tags []string, query string) (string, error) {
	return Default().RenderStream(context.Background(), tags, query)
}

// ListNotes fetches the log-view list of notes.
func ListNotes(tags []string, query string) ([]model.SubNote, error) {
	return Default().ListNotes(context.Background(), tags, query, 0, 0, "")
}

// DeleteNote sends a DELETE request for the given note ID.
func DeleteNote(id string) error {
	return Default().DeleteNote(context.Background(), id)
}

// ListTagsDetailed fetches all tags with status and note counts.
func ListTagsDetailed() ([]model.TagInfo, error) {
	return Default().ListTagsDetailed(context.Background())
}

// ApproveTag sends a PUT to approve a tag.
func ApproveTag(name string) error {
	return Default().ApproveTag(context.Background(), name)
}

// RenameTag sends a PUT to rename/merge a tag.
func RenameTag(oldName, newName string) error {
	return Default().RenameTag(context.Background(), oldName, newName)
}

// DeleteTag sends a DELETE to remove a tag.
func DeleteTag(name string) error {
	return Default().DeleteTag(context.Background(), name)
}
