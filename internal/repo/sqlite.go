package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/runminglu/tag-note/internal/model"

	_ "modernc.org/sqlite"
)

// SQLiteRepo implements Repository using SQLite.
type SQLiteRepo struct {
	db *sql.DB
}

// NewSQLiteRepo opens the SQLite database and runs migrations.
func NewSQLiteRepo(dbPath string) (*SQLiteRepo, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteRepo{db: db}, nil
}

// Close closes the database connection.
func (r *SQLiteRepo) Close() error {
	return r.db.Close()
}

// Ping verifies the database connection is alive.
func (r *SQLiteRepo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// DB returns the underlying *sql.DB for operational queries.
func (r *SQLiteRepo) DB() *sql.DB {
	return r.db
}

func (r *SQLiteRepo) Create(ctx context.Context, userID, id, content string, tags []string, createdAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO subnotes (id, content, created_at, user_id) VALUES (?, ?, ?, ?)`,
		id, content, createdAt.UTC().Format(time.RFC3339Nano), userID)
	if err != nil {
		return fmt.Errorf("insert subnote: %w", err)
	}

	// Index in FTS
	_, err = tx.ExecContext(ctx,
		`INSERT INTO subnotes_fts (id, content) VALUES (?, ?)`, id, content)
	if err != nil {
		return fmt.Errorf("insert fts: %w", err)
	}

	for _, tagName := range tags {
		_, err = tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO tags (name, status, user_id) VALUES (?, 'unreviewed', ?)`, tagName, userID)
		if err != nil {
			return fmt.Errorf("upsert tag %q: %w", tagName, err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO subnote_tags (subnote_id, tag_id)
			 SELECT ?, id FROM tags WHERE name = ? AND user_id = ?`, id, tagName, userID)
		if err != nil {
			return fmt.Errorf("link tag %q: %w", tagName, err)
		}
	}

	return tx.Commit()
}

func (r *SQLiteRepo) CreateImported(ctx context.Context, userID, id, content string, tags []string, createdAt time.Time, updatedAt *time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var updatedAtStr *string
	if updatedAt != nil {
		s := updatedAt.UTC().Format(time.RFC3339Nano)
		updatedAtStr = &s
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO subnotes (id, content, created_at, updated_at, user_id) VALUES (?, ?, ?, ?, ?)`,
		id, content, createdAt.UTC().Format(time.RFC3339Nano), updatedAtStr, userID)
	if err != nil {
		return fmt.Errorf("insert subnote: %w", err)
	}

	// Index in FTS
	_, err = tx.ExecContext(ctx,
		`INSERT INTO subnotes_fts (id, content) VALUES (?, ?)`, id, content)
	if err != nil {
		return fmt.Errorf("insert fts: %w", err)
	}

	for _, tagName := range tags {
		_, err = tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO tags (name, status, user_id) VALUES (?, 'unreviewed', ?)`, tagName, userID)
		if err != nil {
			return fmt.Errorf("upsert tag %q: %w", tagName, err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO subnote_tags (subnote_id, tag_id)
			 SELECT ?, id FROM tags WHERE name = ? AND user_id = ?`, id, tagName, userID)
		if err != nil {
			return fmt.Errorf("link tag %q: %w", tagName, err)
		}
	}

	return tx.Commit()
}

func (r *SQLiteRepo) Search(ctx context.Context, userID string, tags []string, query string, limit, offset int, sort string) ([]model.SubNote, error) {
	var qb strings.Builder
	var args []interface{}

	hasTags := len(tags) > 0
	hasQuery := strings.TrimSpace(query) != ""

	if hasQuery {
		escapedQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`

		qb.WriteString("SELECT s.id, s.content, s.created_at, s.updated_at, s.pinned,")
		qb.WriteString(" snippet(subnotes_fts, 1, '[[', ']]', '...', 64)")
		qb.WriteString(" FROM subnotes_fts")
		qb.WriteString(" JOIN subnotes s ON s.id = subnotes_fts.id")

		qb.WriteString(" WHERE subnotes_fts MATCH ?")
		args = append(args, escapedQuery)

		qb.WriteString(" AND s.user_id = ?")
		args = append(args, userID)

		qb.WriteString(" AND s.deleted_at IS NULL")

		if hasTags {
			for _, t := range tags {
				qb.WriteString(" AND EXISTS (SELECT 1 FROM subnote_tags st2 JOIN tags t2 ON st2.tag_id = t2.id WHERE st2.subnote_id = s.id AND t2.name = ? AND t2.user_id = ?)")
				args = append(args, t, userID)
			}
		}
	} else {
		qb.WriteString("SELECT s.id, s.content, s.created_at, s.updated_at, s.pinned FROM subnotes s")

		if hasTags {
			qb.WriteString(" JOIN subnote_tags st ON s.id = st.subnote_id")
			qb.WriteString(" JOIN tags t ON st.tag_id = t.id")

			placeholders := make([]string, len(tags))
			for i, t := range tags {
				placeholders[i] = "?"
				args = append(args, t)
			}
			qb.WriteString(fmt.Sprintf(" WHERE t.name IN (%s) AND s.user_id = ? AND s.deleted_at IS NULL", strings.Join(placeholders, ",")))
			args = append(args, userID)
			qb.WriteString(" GROUP BY s.id")
			args = append(args, len(tags))
			qb.WriteString(" HAVING COUNT(DISTINCT t.id) = ?")
		} else {
			qb.WriteString(" WHERE s.user_id = ? AND s.deleted_at IS NULL")
			args = append(args, userID)
		}
	}

	switch sort {
	case "updated":
		qb.WriteString(" ORDER BY s.pinned DESC, COALESCE(s.updated_at, s.created_at) DESC")
	default:
		qb.WriteString(" ORDER BY s.pinned DESC, s.created_at DESC")
	}

	if limit > 0 {
		qb.WriteString(" LIMIT ?")
		args = append(args, limit)
		if offset > 0 {
			qb.WriteString(" OFFSET ?")
			args = append(args, offset)
		}
	}

	rows, err := r.db.QueryContext(ctx, qb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []model.SubNote
	seen := make(map[string]bool)
	for rows.Next() {
		var n model.SubNote
		var ts string
		var updatedAt sql.NullString
		var pinned int
		if hasQuery {
			var snippet string
			if err := rows.Scan(&n.ID, &n.Content, &ts, &updatedAt, &pinned, &snippet); err != nil {
				return nil, err
			}
			if seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			n.Snippet = snippet
		} else {
			if err := rows.Scan(&n.ID, &n.Content, &ts, &updatedAt, &pinned); err != nil {
				return nil, err
			}
		}
		n.Pinned = pinned != 0
		n.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
		if updatedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, updatedAt.String)
			n.UpdatedAt = &t
		}
		n.ShortID = model.MakeShortID(n.ID)
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Populate tags for all notes in a single batch query
	if len(notes) > 0 {
		placeholders := make([]string, len(notes))
		args := make([]interface{}, len(notes))
		tagMap := make(map[string][]string)
		for i, n := range notes {
			placeholders[i] = "?"
			args[i] = n.ID
			tagMap[n.ID] = []string{}
		}
		tagRows, err := r.db.QueryContext(ctx,
			`SELECT st.subnote_id, t.name FROM tags t
			 JOIN subnote_tags st ON t.id = st.tag_id
			 WHERE st.subnote_id IN (`+strings.Join(placeholders, ",")+`)
			 ORDER BY st.subnote_id, t.name`, args...)
		if err != nil {
			return nil, err
		}
		defer tagRows.Close()
		for tagRows.Next() {
			var noteID, tag string
			if err := tagRows.Scan(&noteID, &tag); err != nil {
				return nil, err
			}
			tagMap[noteID] = append(tagMap[noteID], tag)
		}
		if err := tagRows.Err(); err != nil {
			return nil, err
		}
		for i := range notes {
			notes[i].Tags = tagMap[notes[i].ID]
		}
	}

	return notes, nil
}

func (r *SQLiteRepo) FindByID(ctx context.Context, userID, id string) (*model.SubNote, error) {
	var rows *sql.Rows
	var err error
	if len(id) == 26 {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, content, created_at, updated_at, pinned FROM subnotes WHERE id = ? AND user_id = ?`, id, userID)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, content, created_at, updated_at, pinned FROM subnotes WHERE id LIKE ? AND user_id = ?`, id+"%", userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.SubNote
	for rows.Next() {
		var n model.SubNote
		var ts string
		var updatedAt sql.NullString
		var pinned int
		if err := rows.Scan(&n.ID, &n.Content, &ts, &updatedAt, &pinned); err != nil {
			return nil, err
		}
		n.Pinned = pinned != 0
		n.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
		if updatedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, updatedAt.String)
			n.UpdatedAt = &t
		}
		n.ShortID = model.MakeShortID(n.ID)
		results = append(results, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrNotFound
	}
	if len(results) > 1 {
		return nil, ErrAmbiguousID
	}

	note := &results[0]
	note.Tags = []string{}
	tagRows, err := r.db.QueryContext(ctx,
		`SELECT t.name FROM tags t
		 JOIN subnote_tags st ON t.id = st.tag_id
		 WHERE st.subnote_id = ?
		 ORDER BY t.name`, note.ID)
	if err != nil {
		return nil, err
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var tag string
		if err := tagRows.Scan(&tag); err != nil {
			return nil, err
		}
		note.Tags = append(note.Tags, tag)
	}
	return note, tagRows.Err()
}

// resolveID resolves a full or short ID to the full ID, scoped to a user.
func (r *SQLiteRepo) resolveID(ctx context.Context, userID, id string) (string, error) {
	if len(id) == 26 {
		return id, nil
	}
	var fullID string
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM subnotes WHERE id LIKE ? AND user_id = ?`, id+"%", userID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		if err := rows.Scan(&fullID); err != nil {
			return "", err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if count == 0 {
		return "", ErrNotFound
	}
	if count > 1 {
		return "", ErrAmbiguousID
	}
	return fullID, nil
}

func (r *SQLiteRepo) Delete(ctx context.Context, userID, id string) error {
	fullID, err := r.resolveID(ctx, userID, id)
	if err != nil {
		return err
	}
	// Soft delete: set deleted_at instead of removing the row
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.db.ExecContext(ctx, `UPDATE subnotes SET deleted_at = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`, now, fullID, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteRepo) RestoreNote(ctx context.Context, userID, id string) error {
	fullID, err := r.resolveID(ctx, userID, id)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE subnotes SET deleted_at = NULL WHERE id = ? AND user_id = ?`, fullID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteRepo) ListTrashed(ctx context.Context, userID string) ([]model.SubNote, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, content, created_at, updated_at, pinned FROM subnotes WHERE user_id = ? AND deleted_at IS NOT NULL ORDER BY deleted_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []model.SubNote
	for rows.Next() {
		var n model.SubNote
		var ts string
		var updatedAt sql.NullString
		var pinned int
		if err := rows.Scan(&n.ID, &n.Content, &ts, &updatedAt, &pinned); err != nil {
			return nil, err
		}
		n.Pinned = pinned != 0
		n.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
		if updatedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, updatedAt.String)
			n.UpdatedAt = &t
		}
		n.ShortID = model.MakeShortID(n.ID)
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Populate tags
	if len(notes) > 0 {
		placeholders := make([]string, len(notes))
		args := make([]interface{}, len(notes))
		tagMap := make(map[string][]string)
		for i, n := range notes {
			placeholders[i] = "?"
			args[i] = n.ID
			tagMap[n.ID] = []string{}
		}
		tagRows, err := r.db.QueryContext(ctx,
			`SELECT st.subnote_id, t.name FROM tags t
			 JOIN subnote_tags st ON t.id = st.tag_id
			 WHERE st.subnote_id IN (`+strings.Join(placeholders, ",")+`)
			 ORDER BY st.subnote_id, t.name`, args...)
		if err != nil {
			return nil, err
		}
		defer tagRows.Close()
		for tagRows.Next() {
			var noteID, tag string
			if err := tagRows.Scan(&noteID, &tag); err != nil {
				return nil, err
			}
			tagMap[noteID] = append(tagMap[noteID], tag)
		}
		if err := tagRows.Err(); err != nil {
			return nil, err
		}
		for i := range notes {
			notes[i].Tags = tagMap[notes[i].ID]
		}
	}

	return notes, nil
}

func (r *SQLiteRepo) PurgeNote(ctx context.Context, userID, id string) error {
	fullID, err := r.resolveID(ctx, userID, id)
	if err != nil {
		return err
	}
	r.db.ExecContext(ctx, `DELETE FROM subnotes_fts WHERE id = ?`, fullID)
	res, err := r.db.ExecContext(ctx, `DELETE FROM subnotes WHERE id = ? AND user_id = ? AND deleted_at IS NOT NULL`, fullID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLiteRepo) Update(ctx context.Context, userID, id string, content *string, tags *[]string) error {
	fullID, err := r.resolveID(ctx, userID, id)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339Nano)

	if content != nil {
		_, err = tx.ExecContext(ctx,
			`UPDATE subnotes SET content = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
			*content, now, fullID, userID)
		if err != nil {
			return fmt.Errorf("update content: %w", err)
		}
		// Update FTS index
		_, err = tx.ExecContext(ctx,
			`UPDATE subnotes_fts SET content = ? WHERE id = ?`, *content, fullID)
		if err != nil {
			return fmt.Errorf("update fts: %w", err)
		}
	}

	if tags != nil {
		// If only tags changed, still set updated_at
		if content == nil {
			_, err = tx.ExecContext(ctx,
				`UPDATE subnotes SET updated_at = ? WHERE id = ? AND user_id = ?`, now, fullID, userID)
			if err != nil {
				return fmt.Errorf("update timestamp: %w", err)
			}
		}

		// Remove existing tag links
		_, err = tx.ExecContext(ctx,
			`DELETE FROM subnote_tags WHERE subnote_id = ?`, fullID)
		if err != nil {
			return fmt.Errorf("delete tag links: %w", err)
		}

		// Re-insert new tag links
		for _, tagName := range *tags {
			_, err = tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO tags (name, status, user_id) VALUES (?, 'unreviewed', ?)`, tagName, userID)
			if err != nil {
				return fmt.Errorf("upsert tag %q: %w", tagName, err)
			}
			_, err = tx.ExecContext(ctx,
				`INSERT INTO subnote_tags (subnote_id, tag_id)
				 SELECT ?, id FROM tags WHERE name = ? AND user_id = ?`, fullID, tagName, userID)
			if err != nil {
				return fmt.Errorf("link tag %q: %w", tagName, err)
			}
		}
	}

	return tx.Commit()
}

func (r *SQLiteRepo) ListTags(ctx context.Context, userID string, limit int) ([]string, error) {
	query := `
		SELECT t.name
		FROM tags t
		JOIN subnote_tags st ON t.id = st.tag_id
		JOIN subnotes s ON st.subnote_id = s.id
		WHERE s.user_id = ? AND s.deleted_at IS NULL
		GROUP BY t.id
		ORDER BY MAX(s.created_at) DESC`
	args := []interface{}{userID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *SQLiteRepo) ListTagsDetailed(ctx context.Context, userID string) ([]model.TagInfo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.name, t.status, COUNT(st.subnote_id), t.importance, t.urgency
		FROM tags t
		LEFT JOIN subnote_tags st ON t.id = st.tag_id
		LEFT JOIN subnotes s ON st.subnote_id = s.id AND s.deleted_at IS NULL
		WHERE t.user_id = ?
		GROUP BY t.id
		ORDER BY
			CASE t.status WHEN 'unreviewed' THEN 0 ELSE 1 END,
			MAX(s.created_at) DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []model.TagInfo
	for rows.Next() {
		var t model.TagInfo
		if err := rows.Scan(&t.Name, &t.Status, &t.NoteCount, &t.Importance, &t.Urgency); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *SQLiteRepo) ApproveTag(ctx context.Context, userID, name string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tags SET status = 'approved' WHERE name = ? AND user_id = ?`, name, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTagNotFound
	}
	return nil
}

func (r *SQLiteRepo) ApproveAllTags(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tags SET status = 'approved' WHERE status = 'unreviewed' AND user_id = ?`, userID)
	return err
}

func (r *SQLiteRepo) RenameTag(ctx context.Context, userID, oldName, newName string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var oldID int
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM tags WHERE name = ? AND user_id = ?`, oldName, userID).Scan(&oldID)
	if err == sql.ErrNoRows {
		return ErrTagNotFound
	}
	if err != nil {
		return err
	}

	var newID int
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM tags WHERE name = ? AND user_id = ?`, newName, userID).Scan(&newID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// Simple rename — no conflict
		_, err = tx.ExecContext(ctx,
			`UPDATE tags SET name = ? WHERE id = ?`, newName, oldID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	// Merge: move associations from old tag to new, skip duplicates
	_, err = tx.ExecContext(ctx,
		`UPDATE OR IGNORE subnote_tags SET tag_id = ? WHERE tag_id = ?`, newID, oldID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`DELETE FROM subnote_tags WHERE tag_id = ?`, oldID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`DELETE FROM tags WHERE id = ?`, oldID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRepo) DeleteTag(ctx context.Context, userID, name string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM tags WHERE name = ? AND user_id = ?`, name, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTagNotFound
	}
	return nil
}

func (r *SQLiteRepo) UpdateTagPriority(ctx context.Context, userID, name string, importance, urgency int) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tags SET importance = ?, urgency = ? WHERE name = ? AND user_id = ?`,
		importance, urgency, name, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTagNotFound
	}
	return nil
}

func (r *SQLiteRepo) AutocompleteTags(ctx context.Context, userID, prefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.name
		FROM tags t
		JOIN subnote_tags st ON t.id = st.tag_id
		JOIN subnotes s ON st.subnote_id = s.id
		WHERE t.name LIKE ? || '%' AND s.user_id = ?
		GROUP BY t.id
		ORDER BY MAX(s.created_at) DESC
		LIMIT ?`, prefix, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *SQLiteRepo) TogglePin(ctx context.Context, userID, id string) error {
	fullID, err := r.resolveID(ctx, userID, id)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE subnotes SET pinned = CASE WHEN pinned = 0 THEN 1 ELSE 0 END WHERE id = ? AND user_id = ?`,
		fullID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
