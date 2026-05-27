package apiclient

import (
	"bytes"
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

const defaultBaseURL = "http://localhost:3000"

// BaseURL returns the server URL, checking TAGNOTE_URL env var first.
func BaseURL() string {
	if u := os.Getenv("TAGNOTE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return defaultBaseURL
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// authToken returns the JWT token from TAGNOTE_TOKEN env var.
func authToken() string {
	return os.Getenv("TAGNOTE_TOKEN")
}

// newRequest creates an HTTP request with the Authorization header set if TAGNOTE_TOKEN is available.
func newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if token := authToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

// Login authenticates with the server and returns a JWT token.
func Login(email, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := httpClient.Post(
		BaseURL()+"/api/v1/auth/login",
		"application/json",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed (%d): %s", resp.StatusCode, string(b))
	}
	var result struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Token, nil
}

// CreateNote sends a POST to create a new sub-note.
func CreateNote(content string, tags []string) (*model.CreateResponse, error) {
	body, _ := json.Marshal(model.CreateRequest{
		Content: content,
		Tags:    tags,
	})

	req, err := newRequest(http.MethodPost, BaseURL()+"/api/v1/notes", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, string(b))
	}

	var result model.CreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ReadStream fetches the Markdown stream for the given tags and/or search query.
func ReadStream(tags []string, query string) (string, error) {
	req, err := newRequest(http.MethodGet, BaseURL()+"/api/v1/notes/stream?"+buildQuery(tags, query), nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// ListNotes fetches the log-view list of notes.
func ListNotes(tags []string, query string) ([]model.SubNote, error) {
	req, err := newRequest(http.MethodGet, BaseURL()+"/api/v1/notes?"+buildQuery(tags, query), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()

	var notes []model.SubNote
	if err := json.NewDecoder(resp.Body).Decode(&notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// DeleteNote sends a DELETE request for the given note ID.
func DeleteNote(id string) error {
	req, err := newRequest(http.MethodDelete, BaseURL()+"/api/v1/notes/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("note %s not found", id)
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

func buildQuery(tags []string, query string) string {
	v := url.Values{}
	for _, t := range tags {
		v.Add("tag", t)
	}
	if query != "" {
		v.Set("q", query)
	}
	return v.Encode()
}

// ListTagsDetailed fetches all tags with status and note counts.
func ListTagsDetailed() ([]model.TagInfo, error) {
	req, err := newRequest(http.MethodGet, BaseURL()+"/api/v1/tags/detailed", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	var tags []model.TagInfo
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// ApproveTag sends a PUT to approve a tag.
func ApproveTag(name string) error {
	req, err := newRequest(http.MethodPut,
		BaseURL()+"/api/v1/tags/"+url.PathEscape(name)+"/approve", nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("tag %q not found", name)
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

// RenameTag sends a PUT to rename/merge a tag.
func RenameTag(oldName, newName string) error {
	body, _ := json.Marshal(model.TagRenameRequest{NewName: newName})
	req, err := newRequest(http.MethodPut,
		BaseURL()+"/api/v1/tags/"+url.PathEscape(oldName)+"/rename",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("tag %q not found", oldName)
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

// DeleteTag sends a DELETE to remove a tag.
func DeleteTag(name string) error {
	req, err := newRequest(http.MethodDelete,
		BaseURL()+"/api/v1/tags/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed (is tagnote-server running?): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("tag %q not found", name)
	}
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}
