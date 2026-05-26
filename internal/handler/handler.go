package handler

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"tagnote/internal/middleware"
	"tagnote/internal/model"
	"tagnote/internal/repo"
	"tagnote/internal/service"
)

// Handler holds HTTP route handlers.
type Handler struct {
	svc *service.Service
}

// New creates a new Handler.
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts all API routes on the Fiber app.
func (h *Handler) Register(app *fiber.App, ah *AuthHandler, ih *ImageHandler, auth *service.AuthService, auditMiddleware ...fiber.Handler) {
	api := app.Group("/api/v1")

	// Auth routes — rate limited, no JWT middleware
	authGroup := api.Group("/auth")
	authGroup.Use(limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many attempts. Please try again later.",
			})
		},
	}))
	authGroup.Post("/register", ah.RegisterUser)
	authGroup.Post("/login", ah.Login)
	authGroup.Post("/logout", ah.Logout)
	authGroup.Post("/google", ah.GoogleAuth)
	authGroup.Post("/verify-email", ah.VerifyEmail)
	authGroup.Post("/resend-verification", ah.ResendVerification)
	authGroup.Post("/forgot-password", ah.ForgotPassword)
	authGroup.Post("/reset-password", ah.ResetPassword)
	authGroup.Post("/magic-link", ah.RequestMagicLink)
	authGroup.Post("/verify-magic-link", ah.VerifyMagicLink)

	// Protected routes — JWT middleware applied
	protected := api.Group("", middleware.JWTAuth(auth))

	// Apply optional audit middleware to protected routes
	for _, m := range auditMiddleware {
		protected.Use(m)
	}

	protected.Get("/auth/me", ah.Me)

	protected.Post("/notes", h.CreateNote)
	protected.Get("/notes", h.ListNotes)
	protected.Get("/notes/stream", h.StreamNotes)
	protected.Get("/notes/export", h.ExportNotes)
	protected.Post("/notes/import", h.ImportNotes)
	protected.Get("/notes/trash", h.ListTrashed)
	protected.Get("/notes/:id", h.GetNote)
	protected.Put("/notes/:id", h.UpdateNote)
	protected.Put("/notes/:id/pin", h.TogglePin)
	protected.Put("/notes/:id/restore", h.RestoreNote)
	protected.Delete("/notes/:id/permanent", h.PurgeNote)
	protected.Delete("/notes/:id", h.DeleteNote)
	protected.Get("/tags", h.ListTags)
	protected.Get("/tags/detailed", h.ListTagsDetailed)
	protected.Get("/tags/autocomplete", h.AutocompleteTags)
	protected.Put("/tags/approve-all", h.ApproveAllTags)
	protected.Put("/tags/:name/approve", h.ApproveTag)
	protected.Put("/tags/:name/rename", h.RenameTag)
	protected.Put("/tags/:name/priority", h.UpdateTagPriority)
	protected.Delete("/tags/:name", h.DeleteTag)

	protected.Post("/images", ih.Upload)

	protected.Get("/settings", h.GetSettings)
	protected.Put("/settings", h.SaveSettings)
}

func (h *Handler) CreateNote(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	var req model.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	resp, err := h.svc.CreateNote(c.Context(), userID, req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *Handler) ListNotes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	tags := parseTags(c)
	query := c.Query("q")
	sort := c.Query("sort")
	limit := 0
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
	notes, err := h.svc.ReadNotes(c.Context(), userID, tags, query, limit, offset, sort)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if notes == nil {
		notes = []model.SubNote{}
	}
	return c.JSON(notes)
}

func (h *Handler) StreamNotes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	tags := parseTags(c)
	query := c.Query("q")
	md, err := h.svc.RenderStream(c.Context(), userID, tags, query)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	c.Set("Content-Type", "text/markdown; charset=utf-8")
	return c.SendString(md)
}

func (h *Handler) ExportNotes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	export, err := h.svc.ExportData(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	c.Set("Content-Disposition", "attachment; filename=tagnote-export.json")
	return c.JSON(export)
}

func (h *Handler) ImportNotes(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)

	// Try full import format first
	var fullReq model.FullImportRequest
	if err := c.BodyParser(&fullReq); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	// Detect whether this is an old-style import (has "notes" at top level, no "version")
	// or new full import (has "version" field)
	if fullReq.Version > 0 {
		// Full import format
		totalItems := len(fullReq.Notes) + len(fullReq.Trash)
		if totalItems > 10000 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "too many items (max 10000)",
			})
		}

		preview, result, err := h.svc.ImportData(c.Context(), userID, fullReq)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if fullReq.DryRun {
			return c.JSON(preview)
		}
		return c.JSON(result)
	}

	// Legacy format: { notes: [...], dry_run: bool }
	var req model.ImportRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	if len(req.Notes) == 0 {
		// If fullReq had notes but no version, treat them as legacy notes
		if len(fullReq.Notes) > 0 {
			req.Notes = fullReq.Notes
			req.DryRun = fullReq.DryRun
		} else {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "no notes provided",
			})
		}
	}

	if len(req.Notes) > 5000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "too many notes (max 5000)",
		})
	}

	preview, imported, err := h.svc.ImportNotes(c.Context(), userID, req.Notes, req.DryRun)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if req.DryRun {
		if preview.New == nil {
			preview.New = []model.ImportNote{}
		}
		if preview.Duplicates == nil {
			preview.Duplicates = []model.ImportNote{}
		}
		return c.JSON(preview)
	}

	return c.JSON(model.ImportResultResponse{Imported: imported})
}

func (h *Handler) DeleteNote(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id is required",
		})
	}

	err := h.svc.DeleteNote(c.Context(), userID, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "note not found",
			})
		}
		if errors.Is(err, repo.ErrAmbiguousID) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "ambiguous short id, provide more characters",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) GetNote(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id is required",
		})
	}

	note, err := h.svc.GetNote(c.Context(), userID, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "note not found",
			})
		}
		if errors.Is(err, repo.ErrAmbiguousID) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "ambiguous short id, provide more characters",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(note)
}

func (h *Handler) UpdateNote(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id is required",
		})
	}

	var req model.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}

	note, err := h.svc.UpdateNote(c.Context(), userID, id, req)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "note not found",
			})
		}
		if errors.Is(err, repo.ErrAmbiguousID) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "ambiguous short id, provide more characters",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(note)
}

func (h *Handler) TogglePin(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id is required",
		})
	}
	if err := h.svc.TogglePin(c.Context(), userID, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "note not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) RestoreNote(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id is required",
		})
	}
	if err := h.svc.RestoreNote(c.Context(), userID, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "note not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListTrashed(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	notes, err := h.svc.ListTrashed(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if notes == nil {
		notes = []model.SubNote{}
	}
	return c.JSON(notes)
}

func (h *Handler) PurgeNote(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id is required",
		})
	}
	if err := h.svc.PurgeNote(c.Context(), userID, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "note not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListTags(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	limit := 0
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n >= 0 {
			limit = n
		}
	}

	tags, err := h.svc.ListTags(c.Context(), userID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if tags == nil {
		tags = []string{}
	}
	return c.JSON(tags)
}

func (h *Handler) ListTagsDetailed(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	tags, err := h.svc.ListTagsDetailed(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if tags == nil {
		tags = []model.TagInfo{}
	}
	return c.JSON(tags)
}

func (h *Handler) AutocompleteTags(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	q := c.Query("q")
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	tags, err := h.svc.AutocompleteTags(c.Context(), userID, q, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if tags == nil {
		tags = []string{}
	}
	return c.JSON(tags)
}

func (h *Handler) ApproveTag(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	name := c.Params("name")
	if err := h.svc.ApproveTag(c.Context(), userID, name); err != nil {
		if errors.Is(err, repo.ErrTagNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ApproveAllTags(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	if err := h.svc.ApproveAllTags(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) RenameTag(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	name := c.Params("name")
	var req model.TagRenameRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}
	if err := h.svc.RenameTag(c.Context(), userID, name, req.NewName); err != nil {
		if errors.Is(err, repo.ErrTagNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) DeleteTag(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	name := c.Params("name")
	if err := h.svc.DeleteTag(c.Context(), userID, name); err != nil {
		if errors.Is(err, repo.ErrTagNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) UpdateTagPriority(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	name := c.Params("name")
	var req model.TagPriorityRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}
	importance := 50
	urgency := 50
	if req.Importance != nil {
		importance = *req.Importance
	}
	if req.Urgency != nil {
		urgency = *req.Urgency
	}
	if err := h.svc.UpdateTagPriority(c.Context(), userID, name, importance, urgency); err != nil {
		if errors.Is(err, repo.ErrTagNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "tag not found",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// parseTags extracts repeated "tag" query parameters from the request.
func parseTags(c *fiber.Ctx) []string {
	var tags []string
	c.Context().QueryArgs().VisitAll(func(key, value []byte) {
		if string(key) == "tag" {
			v := strings.TrimSpace(string(value))
			if v != "" {
				tags = append(tags, v)
			}
		}
	})
	return tags
}

func (h *Handler) GetSettings(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	settings, err := h.svc.GetSettings(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(settings)
}

func (h *Handler) SaveSettings(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	var settings model.Settings
	if err := c.BodyParser(&settings); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body: " + err.Error(),
		})
	}
	if err := h.svc.SaveSettings(c.Context(), userID, settings); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(settings)
}
