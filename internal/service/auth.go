package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/runminglu/tag-note/internal/model"
	"github.com/runminglu/tag-note/internal/repo"
)

// Errors for auth operations.
var (
	ErrEmailNotVerified = errors.New("please verify your email first")
	ErrInvalidToken     = errors.New("invalid or expired token")
	ErrNoPassword       = errors.New("account does not have a password, please use Google login")
	ErrEmailDisabled    = errors.New("email delivery is not configured")
)

// AuthService handles user registration, login, and JWT tokens.
type AuthService struct {
	repo           repo.Repository
	jwtSecret      []byte
	emailService   *EmailService
	googleClientID string
}

// NewAuth creates a new AuthService.
func NewAuth(r repo.Repository, emailService *EmailService) (*AuthService, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		if os.Getenv("TAGNOTE_TEST_MODE") == "1" || os.Getenv("TAGNOTE_ALLOW_DEV_SECRET") == "1" {
			secret = "tagnote-dev-secret"
		} else {
			return nil, fmt.Errorf("JWT_SECRET is required; set TAGNOTE_ALLOW_DEV_SECRET=1 only for local development")
		}
	}
	return &AuthService{
		repo:           r,
		jwtSecret:      []byte(secret),
		emailService:   emailService,
		googleClientID: os.Getenv("GOOGLE_CLIENT_ID"),
	}, nil
}

// generateSecureToken generates a cryptographically secure random token.
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Register creates a new user and sends a verification email.
func (a *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if len(req.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = email
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	id := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()

	if err := a.repo.CreateUser(ctx, id, email, string(hash), displayName, now); err != nil {
		return nil, err
	}

	a.seedOnboardingContent(ctx, id)

	// Generate verification token if email service is enabled
	if a.emailService.IsEnabled() {
		token, err := generateSecureToken()
		if err != nil {
			return nil, fmt.Errorf("generate token: %w", err)
		}

		tokenID := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()
		expiresAt := now.Add(24 * time.Hour)

		if err := a.repo.CreateEmailVerificationToken(ctx, tokenID, id, token, expiresAt); err != nil {
			return nil, fmt.Errorf("store verification token: %w", err)
		}

		if err := a.emailService.SendVerificationEmail(email, token); err != nil {
			return nil, fmt.Errorf("send verification email: %w", err)
		}

		return &model.AuthResponse{
			User:               model.User{ID: id, Email: email, DisplayName: displayName, CreatedAt: now},
			PendingVerify:      true,
			PendingVerifyEmail: email,
		}, nil
	}

	// If email service is not enabled, auto-verify and return token
	if err := a.repo.SetEmailVerified(ctx, id, true); err != nil {
		return nil, fmt.Errorf("set email verified: %w", err)
	}

	authToken, err := a.generateToken(id, email)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: authToken,
		User:  model.User{ID: id, Email: email, DisplayName: displayName, CreatedAt: now, EmailVerified: true, HasPassword: true},
	}, nil
}

// VerifyEmail verifies a user's email using a token.
func (a *AuthService) VerifyEmail(ctx context.Context, req model.VerifyEmailRequest) (*model.AuthResponse, error) {
	userID, err := a.repo.FindEmailVerificationToken(ctx, req.Token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, repo.ErrTokenExpired) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	if err := a.repo.SetEmailVerified(ctx, userID, true); err != nil {
		return nil, fmt.Errorf("set email verified: %w", err)
	}

	if err := a.repo.DeleteEmailVerificationTokens(ctx, userID); err != nil {
		return nil, fmt.Errorf("delete verification tokens: %w", err)
	}

	user, err := a.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	token, err := a.generateToken(userID, user.Email)
	if err != nil {
		return nil, err
	}

	user.EmailVerified = true
	return &model.AuthResponse{Token: token, User: *user}, nil
}

// ResendVerification resends a verification email to the user.
func (a *AuthService) ResendVerification(ctx context.Context, req model.ResendVerificationRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, _, err := a.repo.FindUserByEmail(ctx, email)
	if err != nil {
		// Silently succeed for security
		return nil
	}

	if user.EmailVerified {
		return nil
	}

	if err := a.repo.DeleteEmailVerificationTokens(ctx, user.ID); err != nil {
		return fmt.Errorf("delete old tokens: %w", err)
	}

	token, err := generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	now := time.Now().UTC()
	tokenID := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()
	expiresAt := now.Add(24 * time.Hour)

	if err := a.repo.CreateEmailVerificationToken(ctx, tokenID, user.ID, token, expiresAt); err != nil {
		return fmt.Errorf("store verification token: %w", err)
	}

	if err := a.emailService.SendVerificationEmail(email, token); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}

	return nil
}

// Login authenticates a user and returns an auth token.
func (a *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, hash, err := a.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if user has a password
	if hash == "" {
		return nil, ErrNoPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check email verification (if email service is enabled)
	if a.emailService.IsEnabled() && !user.EmailVerified {
		return &model.AuthResponse{
			User:               *user,
			PendingVerify:      true,
			PendingVerifyEmail: email,
		}, nil
	}

	token, err := a.generateToken(user.ID, email)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{Token: token, User: *user}, nil
}

// GoogleLogin handles Google OAuth authentication.
func (a *AuthService) GoogleLogin(ctx context.Context, req model.GoogleAuthRequest) (*model.AuthResponse, error) {
	if a.googleClientID == "" {
		return nil, fmt.Errorf("Google login is not configured")
	}

	// Verify the Google ID token
	googleUser, err := a.verifyGoogleToken(req.IDToken)
	if err != nil {
		return nil, fmt.Errorf("invalid Google token: %w", err)
	}

	// Check if user exists by Google ID
	user, err := a.repo.FindUserByGoogleID(ctx, googleUser.Sub)
	if err == nil {
		// User exists with this Google ID - login
		token, err := a.generateToken(user.ID, user.Email)
		if err != nil {
			return nil, err
		}
		return &model.AuthResponse{Token: token, User: *user}, nil
	}

	// Check if user exists by email
	user, _, err = a.repo.FindUserByEmail(ctx, googleUser.Email)
	if err == nil {
		// User exists with this email - link Google ID and login
		if err := a.repo.LinkGoogleID(ctx, user.ID, googleUser.Sub); err != nil {
			return nil, fmt.Errorf("link Google ID: %w", err)
		}
		// Also verify email if not already verified
		if !user.EmailVerified {
			if err := a.repo.SetEmailVerified(ctx, user.ID, true); err != nil {
				return nil, fmt.Errorf("set email verified: %w", err)
			}
		}
		user.HasGoogle = true
		user.EmailVerified = true
		token, err := a.generateToken(user.ID, user.Email)
		if err != nil {
			return nil, err
		}
		return &model.AuthResponse{Token: token, User: *user}, nil
	}

	// Create new user with Google (email auto-verified)
	now := time.Now().UTC()
	id := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()

	displayName := googleUser.Name
	if displayName == "" {
		displayName = googleUser.Email
	}

	if err := a.repo.CreateUserWithGoogle(ctx, id, googleUser.Email, googleUser.Sub, displayName, now); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	a.seedOnboardingContent(ctx, id)

	token, err := a.generateToken(id, googleUser.Email)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		Token: token,
		User: model.User{
			ID:            id,
			Email:         googleUser.Email,
			DisplayName:   displayName,
			CreatedAt:     now,
			EmailVerified: true,
			HasGoogle:     true,
		},
	}, nil
}

type googleTokenInfo struct {
	Sub           string      `json:"sub"`
	Email         string      `json:"email"`
	Name          string      `json:"name"`
	Aud           string      `json:"aud"`
	EmailVerified interface{} `json:"email_verified"`
}

func (a *AuthService) verifyGoogleToken(idToken string) (*googleTokenInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token")
	}

	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	// Verify the audience matches our client ID (if configured)
	if a.googleClientID != "" && info.Aud != a.googleClientID {
		return nil, fmt.Errorf("token audience mismatch")
	}

	if info.Sub == "" || info.Email == "" {
		return nil, fmt.Errorf("token missing account identity")
	}

	if !info.isEmailVerified() {
		return nil, fmt.Errorf("Google email is not verified")
	}

	return &info, nil
}

func (i googleTokenInfo) isEmailVerified() bool {
	switch value := i.EmailVerified.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}

// ForgotPassword initiates a password reset.
func (a *AuthService) ForgotPassword(ctx context.Context, req model.ForgotPasswordRequest) error {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	user, _, err := a.repo.FindUserByEmail(ctx, email)
	if err != nil {
		// Silently succeed for security
		return nil
	}

	if err := a.repo.DeletePasswordResetTokens(ctx, user.ID); err != nil {
		return fmt.Errorf("delete old tokens: %w", err)
	}

	token, err := generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	now := time.Now().UTC()
	tokenID := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()
	expiresAt := now.Add(1 * time.Hour)

	if err := a.repo.CreatePasswordResetToken(ctx, tokenID, user.ID, token, expiresAt); err != nil {
		return fmt.Errorf("store reset token: %w", err)
	}

	if err := a.emailService.SendPasswordResetEmail(email, token); err != nil {
		return fmt.Errorf("send reset email: %w", err)
	}

	return nil
}

// ResetPassword resets a user's password using a token.
func (a *AuthService) ResetPassword(ctx context.Context, req model.ResetPasswordRequest) error {
	if len(req.NewPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	userID, err := a.repo.FindPasswordResetToken(ctx, req.Token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, repo.ErrTokenExpired) {
			return ErrInvalidToken
		}
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := a.repo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if err := a.repo.DeletePasswordResetTokens(ctx, userID); err != nil {
		return fmt.Errorf("delete reset tokens: %w", err)
	}

	return nil
}

// RequestMagicLink sends a magic link for passwordless login or registration.
func (a *AuthService) RequestMagicLink(ctx context.Context, req model.MagicLinkRequest) error {
	if !a.emailService.IsEnabled() {
		return ErrEmailDisabled
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return fmt.Errorf("email is required")
	}

	user, _, err := a.repo.FindUserByEmail(ctx, email)
	if err != nil {
		// User doesn't exist - create a new account
		now := time.Now().UTC()
		id := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()

		// Use email as display name initially (can be changed later)
		displayName := strings.Split(email, "@")[0]

		// Create user without password
		if err := a.repo.CreateUserWithoutPassword(ctx, id, email, displayName, now); err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		a.seedOnboardingContent(ctx, id)

		user = &model.User{
			ID:          id,
			Email:       email,
			DisplayName: displayName,
			CreatedAt:   now,
		}
	}

	// Delete any existing magic link tokens for this user
	if err := a.repo.DeleteMagicLinkTokens(ctx, user.ID); err != nil {
		return fmt.Errorf("delete old tokens: %w", err)
	}

	token, err := generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}

	now := time.Now().UTC()
	tokenID := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()
	expiresAt := now.Add(15 * time.Minute)

	if err := a.repo.CreateMagicLinkToken(ctx, tokenID, user.ID, token, expiresAt); err != nil {
		return fmt.Errorf("store magic link token: %w", err)
	}

	if err := a.emailService.SendMagicLinkEmail(email, token); err != nil {
		return fmt.Errorf("send magic link email: %w", err)
	}

	return nil
}

// VerifyMagicLink verifies a magic link token and returns an auth token.
func (a *AuthService) VerifyMagicLink(ctx context.Context, req model.VerifyMagicLinkRequest) (*model.AuthResponse, error) {
	userID, err := a.repo.FindMagicLinkToken(ctx, req.Token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, repo.ErrTokenExpired) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	// Delete the token (one-time use)
	if err := a.repo.DeleteMagicLinkTokens(ctx, userID); err != nil {
		return nil, fmt.Errorf("delete magic link tokens: %w", err)
	}

	// Auto-verify email if not already verified
	if err := a.repo.SetEmailVerified(ctx, userID, true); err != nil {
		return nil, fmt.Errorf("set email verified: %w", err)
	}

	user, err := a.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	token, err := a.generateToken(userID, user.Email)
	if err != nil {
		return nil, err
	}

	user.EmailVerified = true
	return &model.AuthResponse{Token: token, User: *user}, nil
}

// GetUser returns user info by ID.
func (a *AuthService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	return a.repo.FindUserByID(ctx, userID)
}

// ValidateToken parses a JWT and returns the user ID from claims.
func (a *AuthService) ValidateToken(tokenStr string) (userID string, err error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return a.jwtSecret, nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	sub, ok := claims["sub"].(string)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}
	return sub, nil
}

// EnsureTestUser creates the test user if it doesn't already exist.
func (a *AuthService) EnsureTestUser(ctx context.Context) error {
	const testEmail = "test@test.com"
	const testPassword = "testpass123"
	const testDisplayName = "Test User"

	_, _, err := a.repo.FindUserByEmail(ctx, testEmail)
	if err == nil {
		return nil // already exists
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	now := time.Now().UTC()
	id := ulid.MustNew(ulid.Timestamp(now), rand.Reader).String()
	if err := a.repo.CreateUser(ctx, id, testEmail, string(hash), testDisplayName, now); err != nil {
		return err
	}
	// Auto-verify the test user
	if err := a.repo.SetEmailVerified(ctx, id, true); err != nil {
		return err
	}
	a.seedOnboardingContent(ctx, id)
	return nil
}

func (a *AuthService) generateToken(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

// constantTimeCompare performs a constant-time comparison of two strings.
// This is not currently used but available for future token comparisons.
func constantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
