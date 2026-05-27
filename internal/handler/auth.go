package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/runminglu/tag-note/internal/model"
	"github.com/runminglu/tag-note/internal/service"
)

// AuthHandler holds auth-related HTTP route handlers.
type AuthHandler struct {
	auth *service.AuthService
}

// NewAuth creates a new AuthHandler.
func NewAuth(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// RegisterUser handles POST /api/v1/auth/register.
func (h *AuthHandler) RegisterUser(c *fiber.Ctx) error {
	var req model.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	resp, err := h.auth.Register(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	resp, err := h.auth.Login(c.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrNoPassword) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(resp)
}

// Logout handles POST /api/v1/auth/logout (no-op for JWT).
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "logged out"})
}

// Me handles GET /api/v1/auth/me.
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	user, err := h.auth.GetUser(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}
	return c.JSON(user)
}

// GoogleAuth handles POST /api/v1/auth/google.
func (h *AuthHandler) GoogleAuth(c *fiber.Ctx) error {
	var req model.GoogleAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.IDToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id_token is required",
		})
	}
	resp, err := h.auth.GoogleLogin(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(resp)
}

// VerifyEmail handles POST /api/v1/auth/verify-email.
func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
	var req model.VerifyEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "token is required",
		})
	}
	resp, err := h.auth.VerifyEmail(c.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid or expired verification link",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(resp)
}

// ResendVerification handles POST /api/v1/auth/resend-verification.
func (h *AuthHandler) ResendVerification(c *fiber.Ctx) error {
	var req model.ResendVerificationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email is required",
		})
	}
	if err := h.auth.ResendVerification(c.Context(), req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{"message": "if an account exists with this email, a verification email has been sent"})
}

// ForgotPassword handles POST /api/v1/auth/forgot-password.
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var req model.ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email is required",
		})
	}
	if err := h.auth.ForgotPassword(c.Context(), req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{"message": "if an account exists with this email, a password reset link has been sent"})
}

// ResetPassword handles POST /api/v1/auth/reset-password.
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req model.ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "token is required",
		})
	}
	if req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "new_password is required",
		})
	}
	if err := h.auth.ResetPassword(c.Context(), req); err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid or expired reset link",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{"message": "password has been reset successfully"})
}

// RequestMagicLink handles POST /api/v1/auth/magic-link.
func (h *AuthHandler) RequestMagicLink(c *fiber.Ctx) error {
	var req model.MagicLinkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email is required",
		})
	}
	if err := h.auth.RequestMagicLink(c.Context(), req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(fiber.Map{"message": "if an account exists with this email, a login link has been sent"})
}

// VerifyMagicLink handles POST /api/v1/auth/verify-magic-link.
func (h *AuthHandler) VerifyMagicLink(c *fiber.Ctx) error {
	var req model.VerifyMagicLinkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}
	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "token is required",
		})
	}
	resp, err := h.auth.VerifyMagicLink(c.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid or expired login link",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	return c.JSON(resp)
}
