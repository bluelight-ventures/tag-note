package mcpoauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Client is an OAuth client registered for MCP access.
type Client struct {
	ID                      string
	Name                    string
	RedirectURIs            []string
	Scope                   string
	TokenEndpointAuthMethod string
	CreatedAt               time.Time
}

// AuthorizationCode stores a one-time OAuth authorization code.
type AuthorizationCode struct {
	ClientID            string
	UserID              string
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	ExpiresAt           time.Time
}

// TokenRecord is a persisted opaque OAuth token.
type TokenRecord struct {
	ClientID  string
	UserID    string
	Scope     string
	ExpiresAt time.Time
}

// Store persists MCP OAuth state in SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) RegisterClient(ctx context.Context, c Client) error {
	redirectURIs, err := json.Marshal(c.RedirectURIs)
	if err != nil {
		return fmt.Errorf("marshal redirect uris: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO oauth_clients (client_id, client_name, redirect_uris, scope, token_endpoint_auth_method, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, string(redirectURIs), c.Scope, c.TokenEndpointAuthMethod, c.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) FindClient(ctx context.Context, clientID string) (*Client, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT client_id, client_name, redirect_uris, scope, token_endpoint_auth_method, created_at
		FROM oauth_clients WHERE client_id = ?`, clientID)
	var c Client
	var redirectURIs, createdAt string
	if err := row.Scan(&c.ID, &c.Name, &redirectURIs, &c.Scope, &c.TokenEndpointAuthMethod, &createdAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(redirectURIs), &c.RedirectURIs); err != nil {
		return nil, fmt.Errorf("unmarshal redirect uris: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse client created_at: %w", err)
	}
	c.CreatedAt = t
	return &c, nil
}

func (s *Store) CreateAuthorizationCode(ctx context.Context, code string, c AuthorizationCode, createdAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_authorization_codes (
			code_hash, client_id, user_id, redirect_uri, scope, code_challenge,
			code_challenge_method, expires_at, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tokenHash(code), c.ClientID, c.UserID, c.RedirectURI, c.Scope, c.CodeChallenge,
		c.CodeChallengeMethod, c.ExpiresAt.UTC().Format(time.RFC3339Nano), createdAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ConsumeAuthorizationCode(ctx context.Context, code string) (*AuthorizationCode, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	codeHash := tokenHash(code)
	row := tx.QueryRowContext(ctx, `
		SELECT client_id, user_id, redirect_uri, scope, code_challenge, code_challenge_method, expires_at
		FROM oauth_authorization_codes WHERE code_hash = ?`, codeHash)
	var c AuthorizationCode
	var expiresAt string
	if err := row.Scan(&c.ClientID, &c.UserID, &c.RedirectURI, &c.Scope, &c.CodeChallenge, &c.CodeChallengeMethod, &expiresAt); err != nil {
		return nil, err
	}
	if c.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt); err != nil {
		return nil, fmt.Errorf("parse code expires_at: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_authorization_codes WHERE code_hash = ?`, codeHash); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) CreateAccessToken(ctx context.Context, token string, rec TokenRecord, createdAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_access_tokens (token_hash, client_id, user_id, scope, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		tokenHash(token), rec.ClientID, rec.UserID, rec.Scope,
		rec.ExpiresAt.UTC().Format(time.RFC3339Nano), createdAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) FindAccessToken(ctx context.Context, token string) (*TokenRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT client_id, user_id, scope, expires_at
		FROM oauth_access_tokens WHERE token_hash = ?`, tokenHash(token))
	return scanTokenRecord(row)
}

func (s *Store) CreateRefreshToken(ctx context.Context, token string, rec TokenRecord, createdAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth_refresh_tokens (token_hash, client_id, user_id, scope, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		tokenHash(token), rec.ClientID, rec.UserID, rec.Scope,
		rec.ExpiresAt.UTC().Format(time.RFC3339Nano), createdAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) RotateRefreshToken(ctx context.Context, oldToken, newToken string, rec TokenRecord, createdAt time.Time) (*TokenRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var revokedAt sql.NullString
	var expiresAt string
	row := tx.QueryRowContext(ctx, `
		SELECT client_id, user_id, scope, expires_at, revoked_at
		FROM oauth_refresh_tokens WHERE token_hash = ?`, tokenHash(oldToken))
	var existing TokenRecord
	if err := row.Scan(&existing.ClientID, &existing.UserID, &existing.Scope, &expiresAt, &revokedAt); err != nil {
		return nil, err
	}
	exp, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse refresh expires_at: %w", err)
	}
	if revokedAt.Valid || exp.Before(time.Now()) {
		return nil, sql.ErrNoRows
	}
	if existing.ClientID != rec.ClientID {
		return nil, sql.ErrNoRows
	}

	now := createdAt.UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at = ? WHERE token_hash = ?`, now, tokenHash(oldToken)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO oauth_refresh_tokens (token_hash, client_id, user_id, scope, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		tokenHash(newToken), existing.ClientID, existing.UserID, existing.Scope,
		rec.ExpiresAt.UTC().Format(time.RFC3339Nano), now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *Store) RevokeRefreshToken(ctx context.Context, token string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE oauth_refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL`,
		at.UTC().Format(time.RFC3339Nano), tokenHash(token))
	return err
}

func scanTokenRecord(row *sql.Row) (*TokenRecord, error) {
	var rec TokenRecord
	var expiresAt string
	if err := row.Scan(&rec.ClientID, &rec.UserID, &rec.Scope, &expiresAt); err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse token expires_at: %w", err)
	}
	rec.ExpiresAt = t
	return &rec, nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
