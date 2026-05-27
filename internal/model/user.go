package model

import "time"

// User represents a registered user.
type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	CreatedAt     time.Time `json:"created_at"`
	EmailVerified bool      `json:"email_verified"`
	HasPassword   bool      `json:"has_password"`
	HasGoogle     bool      `json:"has_google"`
}

// RegisterRequest is the input for user registration.
type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// LoginRequest is the input for user login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned after login or registration.
type AuthResponse struct {
	Token              string `json:"token,omitempty"`
	User               User   `json:"user"`
	PendingVerify      bool   `json:"pending_verify,omitempty"`
	PendingVerifyEmail string `json:"pending_verify_email,omitempty"`
}

// GoogleAuthRequest is the input for Google OAuth authentication.
type GoogleAuthRequest struct {
	IDToken string `json:"id_token"`
}

// ForgotPasswordRequest is the input for password reset request.
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the input for resetting the password.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// VerifyEmailRequest is the input for email verification.
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// ResendVerificationRequest is the input for resending verification email.
type ResendVerificationRequest struct {
	Email string `json:"email"`
}

// MagicLinkRequest is the input for requesting a magic link login.
type MagicLinkRequest struct {
	Email string `json:"email"`
}

// VerifyMagicLinkRequest is the input for verifying a magic link token.
type VerifyMagicLinkRequest struct {
	Token string `json:"token"`
}
