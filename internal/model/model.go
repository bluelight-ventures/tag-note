package model

import "time"

// SubNote is the core domain entity.
type SubNote struct {
	ID        string     `json:"id"`
	ShortID   string     `json:"short_id"`
	Content   string     `json:"content"`
	Snippet   string     `json:"snippet,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Tags      []string   `json:"tags"`
	Pinned    bool       `json:"pinned"`
}

// ShortIDLen is the number of characters used for short IDs.
const ShortIDLen = 10

// MakeShortID returns the first ShortIDLen characters of an ID.
func MakeShortID(id string) string {
	if len(id) <= ShortIDLen {
		return id
	}
	return id[:ShortIDLen]
}

// CreateRequest is the input for creating a sub-note.
type CreateRequest struct {
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

// CreateResponse is returned after creating a sub-note.
type CreateResponse struct {
	ID        string    `json:"id"`
	ShortID   string    `json:"short_id"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateRequest is the input for updating a sub-note.
// Nil fields are left unchanged.
type UpdateRequest struct {
	Content *string   `json:"content,omitempty"`
	Tags    *[]string `json:"tags,omitempty"`
}

// TagInfo represents a tag with its metadata for management views.
type TagInfo struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	NoteCount  int    `json:"note_count"`
	Importance int    `json:"importance"`
	Urgency    int    `json:"urgency"`
}

// TagRenameRequest is the input for renaming a tag.
type TagRenameRequest struct {
	NewName string `json:"new_name"`
}

// TagPriorityRequest is the input for updating tag importance/urgency.
type TagPriorityRequest struct {
	Importance *int `json:"importance"`
	Urgency    *int `json:"urgency"`
}

// ImportNote represents a single note in an import payload.
type ImportNote struct {
	Content   string     `json:"content"`
	Tags      []string   `json:"tags"`
	Pinned    bool       `json:"pinned"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

// ImportRequest is the input for bulk-importing notes.
type ImportRequest struct {
	Notes  []ImportNote `json:"notes"`
	DryRun bool         `json:"dry_run"`
}

// ImportPreviewResponse is returned when dry_run is true.
type ImportPreviewResponse struct {
	New        []ImportNote `json:"new"`
	Duplicates []ImportNote `json:"duplicates"`
}

// ImportResultResponse is returned when dry_run is false.
type ImportResultResponse struct {
	Imported int `json:"imported"`
}

// FullExport is the comprehensive export format containing all user data.
type FullExport struct {
	Version  int       `json:"version"`
	Exported time.Time `json:"exported_at"`
	Notes    []SubNote `json:"notes"`
	Trash    []SubNote `json:"trash"`
	Tags     []TagInfo `json:"tags"`
	Settings Settings  `json:"settings"`
}

// Settings holds user preferences that are persisted server-side.
type Settings struct {
	Theme       string `json:"theme"`
	PreviewMode string `json:"preview_mode"`
	NoteWidth   string `json:"note_width"`
}

// FullImportRequest is the input for importing a full export.
type FullImportRequest struct {
	DryRun bool `json:"dry_run"`

	// The full export payload (when importing a full export file)
	Version  int          `json:"version"`
	Notes    []ImportNote `json:"notes,omitempty"`
	Trash    []ImportNote `json:"trash,omitempty"`
	Tags     []TagInfo    `json:"tags,omitempty"`
	Settings *Settings    `json:"settings,omitempty"`
}

// FullImportPreview is returned when dry_run is true for a full import.
type FullImportPreview struct {
	NewNotes       []ImportNote `json:"new_notes"`
	DuplicateNotes []ImportNote `json:"duplicate_notes"`
	NewTrash       []ImportNote `json:"new_trash"`
	DuplicateTrash []ImportNote `json:"duplicate_trash"`
	NewTags        []TagInfo    `json:"new_tags"`
	UpdatedTags    []TagInfo    `json:"updated_tags"`
	Settings       *Settings    `json:"settings,omitempty"`
}

// FullImportResult is returned when dry_run is false for a full import.
type FullImportResult struct {
	ImportedNotes   int  `json:"imported_notes"`
	ImportedTrash   int  `json:"imported_trash"`
	ImportedTags    int  `json:"imported_tags"`
	SettingsApplied bool `json:"settings_applied"`
}
