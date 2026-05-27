package admin

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"

	"github.com/runminglu/tag-note/internal/repo"
	"github.com/runminglu/tag-note/internal/service"
)

// AdminOnly is a Fiber middleware that restricts access to admin users.
// It extracts the userID from c.Locals("userID") (set by JWT middleware),
// looks up the user's email, and compares it against cfg.AdminEmail.
func AdminOnly(cfg AdminConfig, auth *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !cfg.IsEnabled() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "admin features are not enabled",
			})
		}

		userID, ok := c.Locals("userID").(string)
		if !ok || userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		user, err := auth.GetUser(c.Context(), userID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "user not found",
			})
		}

		if !strings.EqualFold(user.Email, cfg.AdminEmail) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "admin access required",
			})
		}

		return c.Next()
	}
}

// AuditLog is a Fiber middleware that logs admin actions to the audit_logs table.
// It calls c.Next() first, then records the request details in a fire-and-forget goroutine.
func AuditLog(r repo.Repository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		userID, _ := c.Locals("userID").(string)
		if userID == "" {
			return err
		}

		method := c.Method()
		path := c.Path()
		status := c.Response().StatusCode()
		ip := c.IP()
		userAgent := c.Get("User-Agent")

		action := deriveAction(method, path)

		go func() {
			now := time.Now().UTC()
			id := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()
			// Use context.Background() since the request context may be canceled after the response
			r.CreateAuditLog(context.Background(), id, userID, action, method, path, status, ip, userAgent, "", now)
		}()

		return err
	}
}

// deriveAction derives a human-readable action name from the HTTP method and path.
func deriveAction(method, path string) string {
	switch {
	case method == "POST" && strings.HasSuffix(path, "/notes"):
		return "create_note"
	case method == "PUT" && strings.Contains(path, "/notes/"):
		if strings.HasSuffix(path, "/pin") {
			return "toggle_pin"
		}
		if strings.HasSuffix(path, "/restore") {
			return "restore_note"
		}
		return "update_note"
	case method == "DELETE" && strings.Contains(path, "/notes/"):
		if strings.HasSuffix(path, "/permanent") {
			return "purge_note"
		}
		return "delete_note"
	case method == "PUT" && strings.Contains(path, "/tags/"):
		if strings.HasSuffix(path, "/approve") {
			return "approve_tag"
		}
		if strings.HasSuffix(path, "/rename") {
			return "rename_tag"
		}
		if strings.HasSuffix(path, "/priority") {
			return "update_tag_priority"
		}
		if strings.HasSuffix(path, "/approve-all") {
			return "approve_all_tags"
		}
		return "update_tag"
	case method == "DELETE" && strings.Contains(path, "/tags/"):
		return "delete_tag"
	case method == "POST" && strings.HasSuffix(path, "/images"):
		return "upload_image"
	case method == "POST" && strings.HasSuffix(path, "/import"):
		return "import_notes"
	case method == "GET" && strings.HasSuffix(path, "/export"):
		return "export_notes"
	case method == "PUT" && strings.HasSuffix(path, "/settings"):
		return "update_settings"
	case method == "GET" && strings.Contains(path, "/admin/"):
		return "admin_view"
	default:
		return method + " " + path
	}
}
