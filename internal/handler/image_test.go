package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/runminglu/tag-note/internal/repo"
	"github.com/runminglu/tag-note/internal/service"
)

func TestImageUploadRecordsAuthenticatedOwner(t *testing.T) {
	t.Setenv("TAGNOTE_ALLOW_DEV_SECRET", "1")

	r, err := repo.NewSQLiteRepo(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewSQLiteRepo() error = %v", err)
	}
	defer r.Close()

	uploadDir := t.TempDir()
	app := fiber.New()
	svc := service.New(r)
	authSvc, err := service.NewAuth(r, service.NewEmailService(), uploadDir)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	New(svc).Register(app, NewAuth(authSvc), NewImage(uploadDir, r), authSvc)

	registerBody := strings.NewReader(`{"email":"upload-owner@example.com","password":"testpass123","display_name":"Upload Owner"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", registerBody)
	registerReq.Header.Set("Content-Type", "application/json")
	registerResp, err := app.Test(registerReq, -1)
	if err != nil {
		t.Fatalf("register request error = %v", err)
	}
	defer registerResp.Body.Close()
	if registerResp.StatusCode != fiber.StatusCreated {
		body, _ := io.ReadAll(registerResp.Body)
		t.Fatalf("register status = %d, body = %s", registerResp.StatusCode, body)
	}
	var registerPayload struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(registerResp.Body).Decode(&registerPayload); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if registerPayload.Token == "" || registerPayload.User.ID == "" {
		t.Fatalf("register response missing token/user: %#v", registerPayload)
	}

	var uploadBody bytes.Buffer
	writer := multipart.NewWriter(&uploadBody)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="note.png"`)
	partHeader.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("CreatePart() error = %v", err)
	}
	if _, err := part.Write([]byte("png-bytes")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/images", &uploadBody)
	uploadReq.Header.Set("Authorization", "Bearer "+registerPayload.Token)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadResp, err := app.Test(uploadReq, -1)
	if err != nil {
		t.Fatalf("upload request error = %v", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode != fiber.StatusCreated {
		body, _ := io.ReadAll(uploadResp.Body)
		t.Fatalf("upload status = %d, body = %s", uploadResp.StatusCode, body)
	}
	var uploadPayload struct {
		Data struct {
			FilePath string `json:"filePath"`
		} `json:"data"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&uploadPayload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	filename := strings.TrimPrefix(uploadPayload.Data.FilePath, "/uploads/")
	if filename == "" || filename == uploadPayload.Data.FilePath {
		t.Fatalf("upload filePath = %q, want /uploads/<filename>", uploadPayload.Data.FilePath)
	}

	var ownerID, contentType string
	var size int64
	if err := r.DB().QueryRow(`SELECT user_id, content_type, size FROM uploads WHERE filename = ?`, filename).
		Scan(&ownerID, &contentType, &size); err != nil {
		t.Fatalf("query upload ownership row: %v", err)
	}
	if ownerID != registerPayload.User.ID {
		t.Fatalf("upload owner = %q, want %q", ownerID, registerPayload.User.ID)
	}
	if contentType != "image/png" {
		t.Fatalf("content_type = %q, want image/png", contentType)
	}
	if size != int64(len("png-bytes")) {
		t.Fatalf("size = %d, want %d", size, len("png-bytes"))
	}
	if _, err := os.Stat(filepath.Join(uploadDir, filename)); err != nil {
		t.Fatalf("uploaded file was not saved: %v", err)
	}
}
