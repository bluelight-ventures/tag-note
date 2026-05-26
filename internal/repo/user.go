package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"tagnote/internal/model"
)

// ErrEmailExists is returned when a registration email is already taken.
var ErrEmailExists = errors.New("email already registered")

// ErrTokenExpired is returned when a token has expired.
var ErrTokenExpired = errors.New("token expired")

// CreateUser inserts a new user row.
func (r *SQLiteRepo) CreateUser(ctx context.Context, id, email, passwordHash, displayName string, createdAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, display_name, created_at, email_verified) VALUES (?, ?, ?, ?, ?, 0)`,
		id, email, passwordHash, displayName, createdAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return ErrEmailExists
		}
		return err
	}
	return nil
}

// FindUserByEmail looks up a user by email, returning the user and password hash.
func (r *SQLiteRepo) FindUserByEmail(ctx context.Context, email string) (*model.User, string, error) {
	var u model.User
	var ts string
	var passwordHash sql.NullString
	var googleID sql.NullString
	var emailVerified int

	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, display_name, created_at, google_id, email_verified FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &passwordHash, &u.DisplayName, &ts, &googleID, &emailVerified)
	if err == sql.ErrNoRows {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}

	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	u.EmailVerified = emailVerified == 1
	u.HasPassword = passwordHash.Valid && passwordHash.String != ""
	u.HasGoogle = googleID.Valid && googleID.String != ""

	return &u, passwordHash.String, nil
}

// FindUserByID looks up a user by ID.
func (r *SQLiteRepo) FindUserByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	var ts string
	var passwordHash sql.NullString
	var googleID sql.NullString
	var emailVerified int

	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, display_name, created_at, password_hash, google_id, email_verified FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.DisplayName, &ts, &passwordHash, &googleID, &emailVerified)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	u.EmailVerified = emailVerified == 1
	u.HasPassword = passwordHash.Valid && passwordHash.String != ""
	u.HasGoogle = googleID.Valid && googleID.String != ""

	return &u, nil
}

// FindUserByGoogleID looks up a user by Google OAuth subject ID.
func (r *SQLiteRepo) FindUserByGoogleID(ctx context.Context, googleID string) (*model.User, error) {
	var u model.User
	var ts string
	var passwordHash sql.NullString
	var gID sql.NullString
	var emailVerified int

	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, display_name, created_at, password_hash, google_id, email_verified FROM users WHERE google_id = ?`, googleID).
		Scan(&u.ID, &u.Email, &u.DisplayName, &ts, &passwordHash, &gID, &emailVerified)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	u.EmailVerified = emailVerified == 1
	u.HasPassword = passwordHash.Valid && passwordHash.String != ""
	u.HasGoogle = gID.Valid && gID.String != ""

	return &u, nil
}

// LinkGoogleID links a Google OAuth subject ID to an existing user.
func (r *SQLiteRepo) LinkGoogleID(ctx context.Context, userID, googleID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET google_id = ? WHERE id = ?`, googleID, userID)
	return err
}

// CreateUserWithGoogle creates a new user using Google OAuth (email auto-verified).
func (r *SQLiteRepo) CreateUserWithGoogle(ctx context.Context, id, email, googleID, displayName string, createdAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, display_name, created_at, google_id, email_verified) VALUES (?, ?, NULL, ?, ?, ?, 1)`,
		id, email, displayName, createdAt.UTC().Format(time.RFC3339Nano), googleID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return ErrEmailExists
		}
		return err
	}
	return nil
}

// CreateUserWithoutPassword creates a new user without a password (for magic link registration).
func (r *SQLiteRepo) CreateUserWithoutPassword(ctx context.Context, id, email, displayName string, createdAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, display_name, created_at, email_verified) VALUES (?, ?, '', ?, ?, 0)`,
		id, email, displayName, createdAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return ErrEmailExists
		}
		return err
	}
	return nil
}

// SetEmailVerified sets the email_verified flag for a user.
func (r *SQLiteRepo) SetEmailVerified(ctx context.Context, userID string, verified bool) error {
	val := 0
	if verified {
		val = 1
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET email_verified = ? WHERE id = ?`, val, userID)
	return err
}

// CreateEmailVerificationToken creates a new email verification token.
func (r *SQLiteRepo) CreateEmailVerificationToken(ctx context.Context, id, userID, token string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO email_verification_tokens (id, user_id, token, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, userID, token, expiresAt.UTC().Format(time.RFC3339Nano), now)
	return err
}

// FindEmailVerificationToken finds a valid (non-expired) email verification token.
func (r *SQLiteRepo) FindEmailVerificationToken(ctx context.Context, token string) (string, error) {
	var userID string
	var expiresAt string

	err := r.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM email_verification_tokens WHERE token = ?`, token).
		Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	expTime, _ := time.Parse(time.RFC3339Nano, expiresAt)
	if time.Now().UTC().After(expTime) {
		return "", ErrTokenExpired
	}

	return userID, nil
}

// DeleteEmailVerificationTokens deletes all email verification tokens for a user.
func (r *SQLiteRepo) DeleteEmailVerificationTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM email_verification_tokens WHERE user_id = ?`, userID)
	return err
}

// CreatePasswordResetToken creates a new password reset token.
func (r *SQLiteRepo) CreatePasswordResetToken(ctx context.Context, id, userID, token string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (id, user_id, token, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, userID, token, expiresAt.UTC().Format(time.RFC3339Nano), now)
	return err
}

// FindPasswordResetToken finds a valid (non-expired) password reset token.
func (r *SQLiteRepo) FindPasswordResetToken(ctx context.Context, token string) (string, error) {
	var userID string
	var expiresAt string

	err := r.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM password_reset_tokens WHERE token = ?`, token).
		Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	expTime, _ := time.Parse(time.RFC3339Nano, expiresAt)
	if time.Now().UTC().After(expTime) {
		return "", ErrTokenExpired
	}

	return userID, nil
}

// DeletePasswordResetTokens deletes all password reset tokens for a user.
func (r *SQLiteRepo) DeletePasswordResetTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
	return err
}

// UpdatePassword updates the password hash for a user.
func (r *SQLiteRepo) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	return err
}

// CreateMagicLinkToken creates a new magic link token for passwordless login.
func (r *SQLiteRepo) CreateMagicLinkToken(ctx context.Context, id, userID, token string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO magic_link_tokens (id, user_id, token, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, userID, token, expiresAt.UTC().Format(time.RFC3339Nano), now)
	return err
}

// FindMagicLinkToken finds a valid (non-expired) magic link token.
func (r *SQLiteRepo) FindMagicLinkToken(ctx context.Context, token string) (string, error) {
	var userID string
	var expiresAt string

	err := r.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM magic_link_tokens WHERE token = ?`, token).
		Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	expTime, _ := time.Parse(time.RFC3339Nano, expiresAt)
	if time.Now().UTC().After(expTime) {
		return "", ErrTokenExpired
	}

	return userID, nil
}

// DeleteMagicLinkTokens deletes all magic link tokens for a user.
func (r *SQLiteRepo) DeleteMagicLinkTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM magic_link_tokens WHERE user_id = ?`, userID)
	return err
}

// GetSettings returns the user's settings. Returns defaults if none saved.
func (r *SQLiteRepo) GetSettings(ctx context.Context, userID string) (*model.Settings, error) {
	var s model.Settings
	err := r.db.QueryRowContext(ctx,
		`SELECT theme, preview_mode, note_width FROM user_settings WHERE user_id = ?`, userID).Scan(&s.Theme, &s.PreviewMode, &s.NoteWidth)
	if err == sql.ErrNoRows {
		return &model.Settings{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveSettings upserts the user's settings.
func (r *SQLiteRepo) SaveSettings(ctx context.Context, userID string, settings model.Settings) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_settings (user_id, theme, preview_mode, note_width)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET theme = excluded.theme, preview_mode = excluded.preview_mode, note_width = excluded.note_width`,
		userID, settings.Theme, settings.PreviewMode, settings.NoteWidth)
	return err
}
