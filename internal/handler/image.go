package handler

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

// ImageHandler holds handlers for image operations.
type ImageHandler struct {
	uploadDir string
}

// NewImage creates a new ImageHandler.
func NewImage(uploadDir string) *ImageHandler {
	return &ImageHandler{uploadDir: uploadDir}
}

// Upload handles POST /api/v1/images.
// Accepts multipart/form-data with a single "file" field.
// Validates: max 5MB, image MIME types only.
// Returns JSON: {"data":{"filePath":"/uploads/<ulid>.<ext>"}}.
func (h *ImageHandler) Upload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no file provided",
		})
	}

	const maxSize = 5 << 20 // 5MB
	if file.Size > maxSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "file too large (max 5MB)",
		})
	}

	ct := file.Header.Get("Content-Type")
	extMap := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}
	ext, ok := extMap[ct]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unsupported image type (allowed: jpeg, png, gif, webp)",
		})
	}

	now := time.Now().UTC()
	id := ulid.MustNew(ulid.Timestamp(now), rand.Reader)
	filename := strings.ToLower(id.String()) + ext

	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "storage error",
		})
	}

	dst := filepath.Join(h.uploadDir, filename)
	if err := c.SaveFile(file, dst); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to save file",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"filePath": "/uploads/" + filename,
		},
	})
}
