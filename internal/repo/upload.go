package repo

import (
	"context"
	"regexp"
	"time"
)

var uploadRefPattern = regexp.MustCompile(`/uploads/([^\s)"'<?#]+)`)
var uploadFilenamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// CreateUpload records ownership for an uploaded file.
func (r *SQLiteRepo) CreateUpload(ctx context.Context, userID, id, filename, contentType string, size int64, createdAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO uploads (id, user_id, filename, content_type, size, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, userID, filename, contentType, size, createdAt.UTC().Format(time.RFC3339Nano))
	return err
}

// ListUserUploadFilenames returns files that should be removed for account
// deletion. It includes tracked uploads owned by the user plus legacy
// /uploads/... references found only in the user's own notes. A legacy
// reference is skipped if any other user owns it (tracked) or references it
// (untracked), so deleting one account never removes a file another account
// still relies on.
func (r *SQLiteRepo) ListUserUploadFilenames(ctx context.Context, userID string) ([]string, error) {
	filenames := map[string]struct{}{}

	// Tracked uploads owned by this user.
	if err := r.queryFilenames(ctx, `SELECT filename FROM uploads WHERE user_id = ?`, []any{userID}, func(filename string) {
		if validUploadFilename(filename) {
			filenames[filename] = struct{}{}
		}
	}); err != nil {
		return nil, err
	}

	// Files owned by or referenced by some other user: never delete these, even
	// if the deleting user's notes also reference them.
	claimedByOthers := map[string]struct{}{}
	if err := r.queryFilenames(ctx, `SELECT filename FROM uploads WHERE user_id != ?`, []any{userID}, func(filename string) {
		claimedByOthers[filename] = struct{}{}
	}); err != nil {
		return nil, err
	}
	if err := r.queryFilenames(ctx, `SELECT content FROM subnotes WHERE user_id != ?`, []any{userID}, func(content string) {
		for _, filename := range extractUploadFilenames(content) {
			claimedByOthers[filename] = struct{}{}
		}
	}); err != nil {
		return nil, err
	}

	// Legacy untracked references in the user's notes, excluding anything
	// another user still owns or references.
	if err := r.queryFilenames(ctx, `SELECT content FROM subnotes WHERE user_id = ?`, []any{userID}, func(content string) {
		for _, filename := range extractUploadFilenames(content) {
			if _, taken := claimedByOthers[filename]; !taken {
				filenames[filename] = struct{}{}
			}
		}
	}); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(filenames))
	for filename := range filenames {
		out = append(out, filename)
	}
	return out, nil
}

// queryFilenames runs query and invokes fn with the first (text) column of each
// row.
func (r *SQLiteRepo) queryFilenames(ctx context.Context, query string, args []any, fn func(string)) error {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return err
		}
		fn(value)
	}
	return rows.Err()
}

func extractUploadFilenames(content string) []string {
	matches := uploadRefPattern.FindAllStringSubmatch(content, -1)
	filenames := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		filename := match[1]
		if !validUploadFilename(filename) {
			continue
		}
		if _, ok := seen[filename]; ok {
			continue
		}
		seen[filename] = struct{}{}
		filenames = append(filenames, filename)
	}
	return filenames
}

func validUploadFilename(filename string) bool {
	return uploadFilenamePattern.MatchString(filename)
}
