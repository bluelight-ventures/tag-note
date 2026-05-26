package repo

import (
	"context"
	"errors"
	"time"

	"tagnote/internal/model"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ErrAmbiguousID is returned when a short ID prefix matches multiple notes.
var ErrAmbiguousID = errors.New("ambiguous short id")

// ErrTagNotFound is returned when a requested tag does not exist.
var ErrTagNotFound = errors.New("tag not found")

// Repository defines the data access contract.
type Repository interface {
	Create(ctx context.Context, userID, id, content string, tags []string, createdAt time.Time) error
	CreateImported(ctx context.Context, userID, id, content string, tags []string, createdAt time.Time, updatedAt *time.Time) error
	Search(ctx context.Context, userID string, tags []string, query string, limit, offset int, sort string) ([]model.SubNote, error)
	FindByID(ctx context.Context, userID, id string) (*model.SubNote, error)
	Delete(ctx context.Context, userID, id string) error
	RestoreNote(ctx context.Context, userID, id string) error
	ListTrashed(ctx context.Context, userID string) ([]model.SubNote, error)
	PurgeNote(ctx context.Context, userID, id string) error
	Update(ctx context.Context, userID, id string, content *string, tags *[]string) error
	ListTags(ctx context.Context, userID string, limit int) ([]string, error)
	ListTagsDetailed(ctx context.Context, userID string) ([]model.TagInfo, error)
	ApproveTag(ctx context.Context, userID, name string) error
	ApproveAllTags(ctx context.Context, userID string) error
	RenameTag(ctx context.Context, userID, oldName, newName string) error
	DeleteTag(ctx context.Context, userID, name string) error
	UpdateTagPriority(ctx context.Context, userID, name string, importance, urgency int) error
	AutocompleteTags(ctx context.Context, userID, prefix string, limit int) ([]string, error)
	TogglePin(ctx context.Context, userID, id string) error

	// User methods
	CreateUser(ctx context.Context, id, email, passwordHash, displayName string, createdAt time.Time) error
	FindUserByEmail(ctx context.Context, email string) (user *model.User, passwordHash string, err error)
	FindUserByID(ctx context.Context, id string) (*model.User, error)

	// Google OAuth methods
	FindUserByGoogleID(ctx context.Context, googleID string) (*model.User, error)
	LinkGoogleID(ctx context.Context, userID, googleID string) error
	CreateUserWithGoogle(ctx context.Context, id, email, googleID, displayName string, createdAt time.Time) error
	CreateUserWithoutPassword(ctx context.Context, id, email, displayName string, createdAt time.Time) error

	// Email verification methods
	SetEmailVerified(ctx context.Context, userID string, verified bool) error
	CreateEmailVerificationToken(ctx context.Context, id, userID, token string, expiresAt time.Time) error
	FindEmailVerificationToken(ctx context.Context, token string) (userID string, err error)
	DeleteEmailVerificationTokens(ctx context.Context, userID string) error

	// Password reset methods
	CreatePasswordResetToken(ctx context.Context, id, userID, token string, expiresAt time.Time) error
	FindPasswordResetToken(ctx context.Context, token string) (userID string, err error)
	DeletePasswordResetTokens(ctx context.Context, userID string) error

	CreateMagicLinkToken(ctx context.Context, id, userID, token string, expiresAt time.Time) error
	FindMagicLinkToken(ctx context.Context, token string) (userID string, err error)
	DeleteMagicLinkTokens(ctx context.Context, userID string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error

	// User settings methods
	GetSettings(ctx context.Context, userID string) (*model.Settings, error)
	SaveSettings(ctx context.Context, userID string, settings model.Settings) error

	// Admin methods
	CreateAuditLog(ctx context.Context, id, userID, action, method, path string, status int, ip, userAgent, detail string, createdAt time.Time) error
	ListAuditLogs(ctx context.Context, userID string, limit, offset int) ([]model.AuditLog, int, error)
	ListAllUsers(ctx context.Context) ([]model.User, error)
	CountUsers(ctx context.Context) (int, error)
	CountActiveUsers(ctx context.Context, since time.Time) (int, error)
	CountNotes(ctx context.Context) (int, error)
}
