package repo

import "database/sql"

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id             TEXT PRIMARY KEY,
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT,
    display_name   TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL,
    google_id      TEXT,
    email_verified INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_token ON email_verification_tokens(token);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);

CREATE TABLE IF NOT EXISTS uploads (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename     TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL,
    size         INTEGER NOT NULL,
    created_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_uploads_user_id ON uploads(user_id);
CREATE INDEX IF NOT EXISTS idx_uploads_filename ON uploads(filename);

CREATE TABLE IF NOT EXISTS subnotes (
    id         TEXT PRIMARY KEY,
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS subnote_tags (
    subnote_id TEXT NOT NULL REFERENCES subnotes(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (subnote_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_subnote_tags_tag_id ON subnote_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_subnote_tags_subnote_id ON subnote_tags(subnote_id);
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);
CREATE INDEX IF NOT EXISTS idx_subnotes_created_at ON subnotes(created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS subnotes_fts USING fts5(id UNINDEXED, content);
`

// Migrate runs schema creation on the database.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// Add updated_at column if missing (for existing databases).
	db.Exec(`ALTER TABLE subnotes ADD COLUMN updated_at TEXT`)

	// Add status column to tags if missing (for existing databases).
	// Existing tags default to 'approved'; new tags are inserted as 'unreviewed'.
	db.Exec(`ALTER TABLE tags ADD COLUMN status TEXT NOT NULL DEFAULT 'approved'`)

	// Populate FTS index for any existing rows not yet indexed.
	db.Exec(`INSERT OR IGNORE INTO subnotes_fts(id, content) SELECT id, content FROM subnotes`)

	// --- Multi-user migration ---

	// Add user_id columns if missing.
	db.Exec(`ALTER TABLE subnotes ADD COLUMN user_id TEXT REFERENCES users(id)`)
	db.Exec(`ALTER TABLE tags ADD COLUMN user_id TEXT REFERENCES users(id)`)

	// Create indexes for user_id.
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_subnotes_user_id ON subnotes(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_tags_user_id ON tags(user_id)`)

	// Create the fake/placeholder user for pre-existing data.
	db.Exec(`INSERT OR IGNORE INTO users (id, email, password_hash, display_name, created_at)
		VALUES ('00000000000000000000000000', 'legacy@placeholder.local',
				'$2a$10$placeholder', 'Legacy Data', datetime('now'))`)

	// Assign existing unowned subnotes to the fake user.
	db.Exec(`UPDATE subnotes SET user_id = '00000000000000000000000000' WHERE user_id IS NULL`)

	// Assign existing unowned tags to the fake user.
	db.Exec(`UPDATE tags SET user_id = '00000000000000000000000000' WHERE user_id IS NULL`)

	// Add importance and urgency columns to tags if missing.
	db.Exec(`ALTER TABLE tags ADD COLUMN importance INTEGER NOT NULL DEFAULT 50`)
	db.Exec(`ALTER TABLE tags ADD COLUMN urgency INTEGER NOT NULL DEFAULT 50`)

	// Rebuild tags table to replace UNIQUE(name) with UNIQUE(user_id, name)
	// so that different users can have tags with the same name.
	migrateTagsUnique(db)

	// Add pinned column to subnotes if missing.
	db.Exec(`ALTER TABLE subnotes ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`)

	// Add deleted_at column to subnotes for soft delete.
	db.Exec(`ALTER TABLE subnotes ADD COLUMN deleted_at TEXT`)

	// --- Auth enhancements migration ---

	// Add google_id column to users if missing (for Google OAuth).
	db.Exec(`ALTER TABLE users ADD COLUMN google_id TEXT`)

	// Add email_verified column to users if missing.
	db.Exec(`ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 0`)

	// Create index on google_id for faster lookups.
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id)`)

	// Add apple_id column to users if missing (for Sign in with Apple).
	db.Exec(`ALTER TABLE users ADD COLUMN apple_id TEXT`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_apple_id ON users(apple_id)`)

	// Auto-verify existing users (they registered before email verification was required).
	db.Exec(`UPDATE users SET email_verified = 1 WHERE email_verified = 0 AND id != '00000000000000000000000000'`)

	// --- User settings migration ---
	db.Exec(`CREATE TABLE IF NOT EXISTS user_settings (
		user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
		theme   TEXT NOT NULL DEFAULT '',
		preview_mode TEXT NOT NULL DEFAULT '',
		note_width TEXT NOT NULL DEFAULT ''
	)`)

	// Add note_width column if it doesn't exist (for existing databases)
	db.Exec(`ALTER TABLE user_settings ADD COLUMN note_width TEXT NOT NULL DEFAULT ''`)

	// --- Admin / audit logs migration ---
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL,
		action     TEXT NOT NULL,
		method     TEXT NOT NULL,
		path       TEXT NOT NULL,
		status     INTEGER NOT NULL,
		ip         TEXT,
		user_agent TEXT,
		detail     TEXT,
		created_at TEXT NOT NULL
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)`)

	// --- Magic link tokens migration ---
	db.Exec(`CREATE TABLE IF NOT EXISTS magic_link_tokens (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token      TEXT NOT NULL UNIQUE,
		expires_at TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_token ON magic_link_tokens(token)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_magic_link_tokens_user_id ON magic_link_tokens(user_id)`)

	// The uploads table backs account-scoped upload ownership and deletion. It
	// is defined in the error-checked schema block above so a failure to create
	// it fails startup.

	return nil
}

// migrateTagsUnique rebuilds the tags table if needed to support per-user
// unique tag names instead of globally unique tag names.
func migrateTagsUnique(db *sql.DB) {
	// Check if user_id column exists and idx_tags_user_name does not.
	var hasUserID int
	row := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tags') WHERE name = 'user_id'`)
	if row.Scan(&hasUserID) != nil || hasUserID == 0 {
		return // user_id column doesn't exist yet; nothing to do
	}

	var hasNewIdx int
	row = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_tags_user_name'`)
	if row.Scan(&hasNewIdx) != nil || hasNewIdx > 0 {
		return // already migrated
	}

	// Disable foreign keys to prevent CASCADE deletes when we drop the old
	// tags table. PRAGMA foreign_keys cannot be changed inside a transaction.
	db.Exec(`PRAGMA foreign_keys = OFF`)

	tx, err := db.Begin()
	if err != nil {
		db.Exec(`PRAGMA foreign_keys = ON`)
		return
	}
	defer tx.Rollback()

	tx.Exec(`CREATE TABLE tags_new (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		status     TEXT NOT NULL DEFAULT 'approved',
		user_id    TEXT REFERENCES users(id),
		importance INTEGER NOT NULL DEFAULT 50,
		urgency    INTEGER NOT NULL DEFAULT 50,
		UNIQUE(user_id, name)
	)`)
	tx.Exec(`INSERT INTO tags_new (id, name, status, user_id, importance, urgency)
		SELECT id, name, status, user_id, COALESCE(importance, 50), COALESCE(urgency, 50) FROM tags`)
	tx.Exec(`DROP TABLE tags`)
	tx.Exec(`ALTER TABLE tags_new RENAME TO tags`)
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name)`)
	tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tags_user_id ON tags(user_id)`)
	tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_user_name ON tags(user_id, name)`)
	tx.Commit()

	// Re-enable foreign keys
	db.Exec(`PRAGMA foreign_keys = ON`)
}
