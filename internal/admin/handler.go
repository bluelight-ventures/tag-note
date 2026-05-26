package admin

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"tagnote/internal/model"
	"tagnote/internal/repo"
	"tagnote/internal/service"
)

// AdminHandler handles admin API endpoints.
type AdminHandler struct {
	repo    repo.Repository
	auth    *service.AuthService
	config  AdminConfig
	db      *sql.DB
	startAt time.Time
}

// NewHandler creates a new AdminHandler.
func NewHandler(r repo.Repository, auth *service.AuthService, cfg AdminConfig, db *sql.DB) *AdminHandler {
	return &AdminHandler{
		repo:    r,
		auth:    auth,
		config:  cfg,
		db:      db,
		startAt: time.Now(),
	}
}

// Overview returns admin dashboard overview statistics.
func (h *AdminHandler) Overview(c *fiber.Ctx) error {
	ctx := c.Context()

	userCount, err := h.repo.CountUsers(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to count users: " + err.Error(),
		})
	}

	notesCount, err := h.repo.CountNotes(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to count notes: " + err.Error(),
		})
	}

	now := time.Now().UTC()
	dau, err := h.repo.CountActiveUsers(ctx, now.Add(-24*time.Hour))
	if err != nil {
		dau = 0
	}

	mau, err := h.repo.CountActiveUsers(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		mau = 0
	}

	var trashCount, tagCount int
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM subnotes WHERE deleted_at IS NOT NULL").Scan(&trashCount)
	h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tags").Scan(&tagCount)

	var pageCount, pageSize int
	h.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount)
	h.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize)
	dbSizeBytes := pageCount * pageSize

	uptime := time.Since(h.startAt)

	overview := model.AdminOverview{
		TotalUsers:  userCount,
		TotalNotes:  notesCount,
		DAU:         dau,
		MAU:         mau,
		UptimeSec:   int(uptime.Seconds()),
		Uptime:      uptime.Truncate(time.Second).String(),
		DBSizeBytes: dbSizeBytes,
		DBSizeMB:    fmt.Sprintf("%.2f", float64(dbSizeBytes)/(1024*1024)),
		TrashCount:  trashCount,
		TagCount:    tagCount,
	}

	UpdateGauges(userCount, dau, notesCount)

	return c.JSON(overview)
}

// Users returns all registered users.
func (h *AdminHandler) Users(c *fiber.Ctx) error {
	users, err := h.repo.ListAllUsers(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list users: " + err.Error(),
		})
	}
	return c.JSON(users)
}

// Logs returns paginated audit logs.
func (h *AdminHandler) Logs(c *fiber.Ctx) error {
	userID := c.Query("user_id")
	limit := 50
	offset := 0

	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := c.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	logs, total, err := h.repo.ListAuditLogs(c.Context(), userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list logs: " + err.Error(),
		})
	}

	return c.JSON(model.AuditLogListResponse{
		Logs:   logs,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
