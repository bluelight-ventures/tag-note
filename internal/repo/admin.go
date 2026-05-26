package repo

import (
	"context"
	"database/sql"
	"time"

	"tagnote/internal/model"
)

// CreateAuditLog inserts a new audit log entry.
func (r *SQLiteRepo) CreateAuditLog(ctx context.Context, id, userID, action, method, path string, status int, ip, userAgent, detail string, createdAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (id, user_id, action, method, path, status, ip, user_agent, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, action, method, path, status, ip, userAgent, detail, createdAt.UTC().Format(time.RFC3339Nano))
	return err
}

// ListAuditLogs returns paginated audit logs, optionally filtered by user ID.
// If userID is empty, returns all logs. Returns logs and total count.
func (r *SQLiteRepo) ListAuditLogs(ctx context.Context, userID string, limit, offset int) ([]model.AuditLog, int, error) {
	var total int
	var countQuery, selectQuery string
	var args []interface{}

	if userID != "" {
		countQuery = `SELECT COUNT(*) FROM audit_logs WHERE user_id = ?`
		selectQuery = `SELECT id, user_id, action, method, path, status, ip, user_agent, detail, created_at
			FROM audit_logs WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
		args = []interface{}{userID, limit, offset}
		if err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total); err != nil {
			return nil, 0, err
		}
	} else {
		countQuery = `SELECT COUNT(*) FROM audit_logs`
		selectQuery = `SELECT id, user_id, action, method, path, status, ip, user_agent, detail, created_at
			FROM audit_logs ORDER BY created_at DESC LIMIT ? OFFSET ?`
		args = []interface{}{limit, offset}
		if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var log model.AuditLog
		var createdAt string
		var ip, userAgent, detail sql.NullString
		if err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Method, &log.Path, &log.Status,
			&ip, &userAgent, &detail, &createdAt); err != nil {
			return nil, 0, err
		}
		log.IP = ip.String
		log.UserAgent = userAgent.String
		log.Detail = detail.String
		log.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		logs = append(logs, log)
	}

	if logs == nil {
		logs = []model.AuditLog{}
	}

	return logs, total, rows.Err()
}

// ListAllUsers returns all registered users (excluding the placeholder user).
func (r *SQLiteRepo) ListAllUsers(ctx context.Context) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, email, display_name, created_at, email_verified, password_hash, google_id
		FROM users
		WHERE id != '00000000000000000000000000'
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		var createdAt string
		var emailVerified int
		var passwordHash, googleID sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &createdAt, &emailVerified, &passwordHash, &googleID); err != nil {
			return nil, err
		}
		u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		u.EmailVerified = emailVerified != 0
		u.HasPassword = passwordHash.Valid && passwordHash.String != ""
		u.HasGoogle = googleID.Valid && googleID.String != ""
		users = append(users, u)
	}

	if users == nil {
		users = []model.User{}
	}

	return users, rows.Err()
}

// CountUsers returns the total number of registered users (excluding placeholder).
func (r *SQLiteRepo) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE id != '00000000000000000000000000'`).Scan(&count)
	return count, err
}

// CountActiveUsers returns the number of distinct users with audit log entries since the given time.
func (r *SQLiteRepo) CountActiveUsers(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM audit_logs WHERE created_at > ?`,
		since.UTC().Format(time.RFC3339Nano)).Scan(&count)
	return count, err
}

// CountNotes returns the total number of non-deleted notes.
func (r *SQLiteRepo) CountNotes(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM subnotes WHERE deleted_at IS NULL`).Scan(&count)
	return count, err
}
