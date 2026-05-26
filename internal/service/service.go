package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"tagnote/internal/model"
	"tagnote/internal/repo"
)

// Service contains business logic for sub-notes.
type Service struct {
	repo repo.Repository
}

// New creates a new Service.
func New(r repo.Repository) *Service {
	return &Service{repo: r}
}

// CreateNote validates input, generates a ULID, and persists a new sub-note.
func (s *Service) CreateNote(ctx context.Context, userID string, req model.CreateRequest) (*model.CreateResponse, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("content must not be empty")
	}
	if len(req.Tags) == 0 {
		return nil, fmt.Errorf("at least one tag is required")
	}

	tags := normalizeTags(req.Tags)
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one valid tag is required")
	}

	now := time.Now().UTC()
	id := ulid.MustNew(ulid.Timestamp(now), rand.Reader)

	idStr := id.String()
	err := s.repo.Create(ctx, userID, idStr, req.Content, tags, now)
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}

	return &model.CreateResponse{
		ID:        idStr,
		ShortID:   model.MakeShortID(idStr),
		CreatedAt: now,
	}, nil
}

// ReadNotes returns sub-notes matching all given tags and/or search query.
func (s *Service) ReadNotes(ctx context.Context, userID string, tags []string, query string, limit, offset int, sort string) ([]model.SubNote, error) {
	return s.repo.Search(ctx, userID, normalizeTags(tags), strings.TrimSpace(query), limit, offset, sort)
}

// RenderStream returns a Markdown string of all matching sub-notes separated by horizontal rules.
func (s *Service) RenderStream(ctx context.Context, userID string, tags []string, query string) (string, error) {
	notes, err := s.ReadNotes(ctx, userID, tags, query, 0, 0, "")
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for i, n := range notes {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(n.Content)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// DeleteNote soft-deletes a sub-note by ID (full or short).
func (s *Service) DeleteNote(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}

// RestoreNote restores a soft-deleted sub-note.
func (s *Service) RestoreNote(ctx context.Context, userID, id string) error {
	return s.repo.RestoreNote(ctx, userID, id)
}

// ListTrashed returns all soft-deleted notes for a user.
func (s *Service) ListTrashed(ctx context.Context, userID string) ([]model.SubNote, error) {
	return s.repo.ListTrashed(ctx, userID)
}

// PurgeNote permanently deletes a soft-deleted note.
func (s *Service) PurgeNote(ctx context.Context, userID, id string) error {
	return s.repo.PurgeNote(ctx, userID, id)
}

// GetNote returns a single sub-note by ID (full or short).
func (s *Service) GetNote(ctx context.Context, userID, id string) (*model.SubNote, error) {
	return s.repo.FindByID(ctx, userID, id)
}

// UpdateNote updates the content and/or tags of a sub-note.
func (s *Service) UpdateNote(ctx context.Context, userID, id string, req model.UpdateRequest) (*model.SubNote, error) {
	if req.Content == nil && req.Tags == nil {
		return nil, fmt.Errorf("nothing to update")
	}
	if req.Content != nil && strings.TrimSpace(*req.Content) == "" {
		return nil, fmt.Errorf("content must not be empty")
	}

	var normalizedTags *[]string
	if req.Tags != nil {
		tags := normalizeTags(*req.Tags)
		if len(tags) == 0 {
			return nil, fmt.Errorf("at least one valid tag is required")
		}
		normalizedTags = &tags
	}

	if err := s.repo.Update(ctx, userID, id, req.Content, normalizedTags); err != nil {
		return nil, err
	}

	return s.repo.FindByID(ctx, userID, id)
}

// ListTags returns tags ordered by most recently used. Limit 0 means all.
func (s *Service) ListTags(ctx context.Context, userID string, limit int) ([]string, error) {
	return s.repo.ListTags(ctx, userID, limit)
}

// ListTagsDetailed returns all tags with status and note counts.
func (s *Service) ListTagsDetailed(ctx context.Context, userID string) ([]model.TagInfo, error) {
	return s.repo.ListTagsDetailed(ctx, userID)
}

// ApproveTag marks a tag as approved.
func (s *Service) ApproveTag(ctx context.Context, userID, name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("tag name must not be empty")
	}
	return s.repo.ApproveTag(ctx, userID, name)
}

// ApproveAllTags marks all unreviewed tags as approved.
func (s *Service) ApproveAllTags(ctx context.Context, userID string) error {
	return s.repo.ApproveAllTags(ctx, userID)
}

// RenameTag renames a tag, merging if the target name already exists.
func (s *Service) RenameTag(ctx context.Context, userID, oldName, newName string) error {
	oldName = strings.ToLower(strings.TrimSpace(oldName))
	newName = strings.ToLower(strings.TrimSpace(newName))
	if oldName == "" || newName == "" {
		return fmt.Errorf("tag names must not be empty")
	}
	if oldName == newName {
		return fmt.Errorf("new name is the same as old name")
	}
	return s.repo.RenameTag(ctx, userID, oldName, newName)
}

// DeleteTag removes a tag and its associations from all notes.
func (s *Service) DeleteTag(ctx context.Context, userID, name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("tag name must not be empty")
	}
	return s.repo.DeleteTag(ctx, userID, name)
}

// UpdateTagPriority updates the importance and urgency of a tag.
func (s *Service) UpdateTagPriority(ctx context.Context, userID, name string, importance, urgency int) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("tag name must not be empty")
	}
	if importance < 0 || importance > 100 {
		return fmt.Errorf("importance must be between 0 and 100")
	}
	if urgency < 0 || urgency > 100 {
		return fmt.Errorf("urgency must be between 0 and 100")
	}
	return s.repo.UpdateTagPriority(ctx, userID, name, importance, urgency)
}

// AutocompleteTags returns tag names matching a prefix, sorted by recency.
func (s *Service) AutocompleteTags(ctx context.Context, userID, prefix string, limit int) ([]string, error) {
	return s.repo.AutocompleteTags(ctx, userID, strings.ToLower(strings.TrimSpace(prefix)), limit)
}

// TogglePin toggles the pinned state of a note.
func (s *Service) TogglePin(ctx context.Context, userID, id string) error {
	return s.repo.TogglePin(ctx, userID, id)
}

// ImportNotes handles bulk note import with deduplication.
// When dryRun is true, returns categorized notes without creating anything.
// When dryRun is false, creates all provided notes and returns the count.
func (s *Service) ImportNotes(ctx context.Context, userID string, notes []model.ImportNote, dryRun bool) (*model.ImportPreviewResponse, int, error) {
	if len(notes) == 0 {
		return nil, 0, fmt.Errorf("no notes to import")
	}

	// Fetch all existing notes for deduplication
	existing, err := s.repo.Search(ctx, userID, nil, "", 0, 0, "")
	if err != nil {
		return nil, 0, fmt.Errorf("fetch existing notes: %w", err)
	}

	// Build a lookup set keyed on trimmed content + sorted normalized tags
	type noteKey struct {
		content string
		tags    string
	}
	existingSet := make(map[noteKey]bool, len(existing))
	for _, n := range existing {
		sorted := make([]string, len(n.Tags))
		copy(sorted, n.Tags)
		sort.Strings(sorted)
		existingSet[noteKey{
			content: strings.TrimSpace(n.Content),
			tags:    strings.Join(sorted, ","),
		}] = true
	}

	var newNotes []model.ImportNote
	var duplicates []model.ImportNote

	for _, n := range notes {
		trimmed := strings.TrimSpace(n.Content)
		if trimmed == "" {
			continue
		}
		tags := normalizeTags(n.Tags)
		if len(tags) == 0 {
			continue
		}

		sorted := make([]string, len(tags))
		copy(sorted, tags)
		sort.Strings(sorted)

		key := noteKey{
			content: trimmed,
			tags:    strings.Join(sorted, ","),
		}

		normalized := model.ImportNote{
			Content:   trimmed,
			Tags:      tags,
			Pinned:    n.Pinned,
			CreatedAt: n.CreatedAt,
			UpdatedAt: n.UpdatedAt,
		}

		if existingSet[key] {
			duplicates = append(duplicates, normalized)
		} else {
			newNotes = append(newNotes, normalized)
			existingSet[key] = true
		}
	}

	if dryRun {
		return &model.ImportPreviewResponse{
			New:        newNotes,
			Duplicates: duplicates,
		}, 0, nil
	}

	imported := 0
	for _, n := range newNotes {
		if err := s.createImportedNote(ctx, userID, n); err != nil {
			return nil, imported, fmt.Errorf("create imported note: %w", err)
		}
		imported++
	}

	return nil, imported, nil
}

// createImportedNote creates a single note from an import, preserving original timestamps if available.
func (s *Service) createImportedNote(ctx context.Context, userID string, n model.ImportNote) error {
	createdAt := time.Now().UTC()
	if n.CreatedAt != nil && !n.CreatedAt.IsZero() {
		createdAt = n.CreatedAt.UTC()
	}
	id := ulid.MustNew(ulid.Timestamp(createdAt), rand.Reader)
	idStr := id.String()

	if err := s.repo.CreateImported(ctx, userID, idStr, n.Content, n.Tags, createdAt, n.UpdatedAt); err != nil {
		return err
	}

	if n.Pinned {
		_ = s.repo.TogglePin(ctx, userID, idStr)
	}
	return nil
}

func normalizeTags(tags []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

// GetSettings returns the user's settings.
func (s *Service) GetSettings(ctx context.Context, userID string) (*model.Settings, error) {
	return s.repo.GetSettings(ctx, userID)
}

// SaveSettings persists the user's settings.
func (s *Service) SaveSettings(ctx context.Context, userID string, settings model.Settings) error {
	return s.repo.SaveSettings(ctx, userID, settings)
}

// ExportData returns a full export containing notes, trash, tags, and settings.
func (s *Service) ExportData(ctx context.Context, userID string) (*model.FullExport, error) {
	notes, err := s.ReadNotes(ctx, userID, nil, "", 0, 0, "")
	if err != nil {
		return nil, fmt.Errorf("export notes: %w", err)
	}
	if notes == nil {
		notes = []model.SubNote{}
	}

	trash, err := s.ListTrashed(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("export trash: %w", err)
	}
	if trash == nil {
		trash = []model.SubNote{}
	}

	tags, err := s.ListTagsDetailed(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("export tags: %w", err)
	}
	if tags == nil {
		tags = []model.TagInfo{}
	}

	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("export settings: %w", err)
	}

	return &model.FullExport{
		Version:  1,
		Exported: time.Now().UTC(),
		Notes:    notes,
		Trash:    trash,
		Tags:     tags,
		Settings: *settings,
	}, nil
}

// ImportData handles full data import with deduplication.
func (s *Service) ImportData(ctx context.Context, userID string, req model.FullImportRequest) (*model.FullImportPreview, *model.FullImportResult, error) {
	// --- Notes deduplication ---
	existing, err := s.repo.Search(ctx, userID, nil, "", 0, 0, "")
	if err != nil {
		return nil, nil, fmt.Errorf("fetch existing notes: %w", err)
	}
	type noteKey struct {
		content string
		tags    string
	}
	existingNoteSet := make(map[noteKey]bool, len(existing))
	for _, n := range existing {
		sorted := make([]string, len(n.Tags))
		copy(sorted, n.Tags)
		sort.Strings(sorted)
		existingNoteSet[noteKey{
			content: strings.TrimSpace(n.Content),
			tags:    strings.Join(sorted, ","),
		}] = true
	}

	classifyNotes := func(notes []model.ImportNote) (newNotes, dups []model.ImportNote) {
		for _, n := range notes {
			trimmed := strings.TrimSpace(n.Content)
			if trimmed == "" {
				continue
			}
			tags := normalizeTags(n.Tags)
			if len(tags) == 0 {
				continue
			}
			sorted := make([]string, len(tags))
			copy(sorted, tags)
			sort.Strings(sorted)
			key := noteKey{content: trimmed, tags: strings.Join(sorted, ",")}
			normalized := model.ImportNote{Content: trimmed, Tags: tags, Pinned: n.Pinned, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt}
			if existingNoteSet[key] {
				dups = append(dups, normalized)
			} else {
				newNotes = append(newNotes, normalized)
				existingNoteSet[key] = true
			}
		}
		return
	}

	newNotes, dupNotes := classifyNotes(req.Notes)

	// --- Trash deduplication ---
	existingTrash, err := s.repo.ListTrashed(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch existing trash: %w", err)
	}
	existingTrashSet := make(map[noteKey]bool, len(existingTrash))
	for _, n := range existingTrash {
		sorted := make([]string, len(n.Tags))
		copy(sorted, n.Tags)
		sort.Strings(sorted)
		existingTrashSet[noteKey{
			content: strings.TrimSpace(n.Content),
			tags:    strings.Join(sorted, ","),
		}] = true
	}

	var newTrash, dupTrash []model.ImportNote
	for _, n := range req.Trash {
		trimmed := strings.TrimSpace(n.Content)
		if trimmed == "" {
			continue
		}
		tags := normalizeTags(n.Tags)
		if len(tags) == 0 {
			continue
		}
		sorted := make([]string, len(tags))
		copy(sorted, tags)
		sort.Strings(sorted)
		key := noteKey{content: trimmed, tags: strings.Join(sorted, ",")}
		normalized := model.ImportNote{Content: trimmed, Tags: tags, Pinned: n.Pinned, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt}
		if existingTrashSet[key] || existingNoteSet[key] {
			dupTrash = append(dupTrash, normalized)
		} else {
			newTrash = append(newTrash, normalized)
			existingTrashSet[key] = true
		}
	}

	// --- Tags classification ---
	existingTags, err := s.repo.ListTagsDetailed(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch existing tags: %w", err)
	}
	existingTagMap := make(map[string]model.TagInfo, len(existingTags))
	for _, t := range existingTags {
		existingTagMap[t.Name] = t
	}

	var newTags, updatedTags []model.TagInfo
	for _, t := range req.Tags {
		t.Name = strings.ToLower(strings.TrimSpace(t.Name))
		if t.Name == "" {
			continue
		}
		if existing, ok := existingTagMap[t.Name]; ok {
			// Tag exists — check if priority differs
			if existing.Importance != t.Importance || existing.Urgency != t.Urgency {
				updatedTags = append(updatedTags, t)
			}
		} else {
			newTags = append(newTags, t)
		}
	}

	if req.DryRun {
		preview := &model.FullImportPreview{
			NewNotes:       newNotes,
			DuplicateNotes: dupNotes,
			NewTrash:       newTrash,
			DuplicateTrash: dupTrash,
			NewTags:        newTags,
			UpdatedTags:    updatedTags,
			Settings:       req.Settings,
		}
		// Ensure no nil slices in preview
		if preview.NewNotes == nil {
			preview.NewNotes = []model.ImportNote{}
		}
		if preview.DuplicateNotes == nil {
			preview.DuplicateNotes = []model.ImportNote{}
		}
		if preview.NewTrash == nil {
			preview.NewTrash = []model.ImportNote{}
		}
		if preview.DuplicateTrash == nil {
			preview.DuplicateTrash = []model.ImportNote{}
		}
		if preview.NewTags == nil {
			preview.NewTags = []model.TagInfo{}
		}
		if preview.UpdatedTags == nil {
			preview.UpdatedTags = []model.TagInfo{}
		}
		return preview, nil, nil
	}

	// --- Actual import ---
	result := &model.FullImportResult{}

	// Import notes
	for _, n := range newNotes {
		if err := s.createImportedNote(ctx, userID, n); err != nil {
			return nil, result, fmt.Errorf("create imported note: %w", err)
		}
		result.ImportedNotes++
	}

	// Import trash (create then soft-delete)
	for _, n := range newTrash {
		createdAt := time.Now().UTC()
		if n.CreatedAt != nil && !n.CreatedAt.IsZero() {
			createdAt = n.CreatedAt.UTC()
		}
		id := ulid.MustNew(ulid.Timestamp(createdAt), rand.Reader)
		idStr := id.String()
		if err := s.repo.CreateImported(ctx, userID, idStr, n.Content, n.Tags, createdAt, n.UpdatedAt); err != nil {
			return nil, result, fmt.Errorf("create imported trash note: %w", err)
		}
		if n.Pinned {
			_ = s.repo.TogglePin(ctx, userID, idStr)
		}
		_ = s.repo.Delete(ctx, userID, idStr)
		result.ImportedTrash++
	}

	// Import/update tags (only priority — tags are auto-created when notes are imported)
	for _, t := range newTags {
		// These tags have no associated notes in the import, but may have custom priority.
		// We still want to preserve them as standalone tags with their metadata.
		if t.Importance != 50 || t.Urgency != 50 || t.Status == "approved" {
			// Create the tag if it doesn't exist yet (it might have been auto-created by note import)
			if err := s.repo.UpdateTagPriority(ctx, userID, t.Name, t.Importance, t.Urgency); err != nil {
				// Tag might not exist yet — that's OK, it will be created when notes reference it
			}
			result.ImportedTags++
		}
	}
	for _, t := range updatedTags {
		if err := s.repo.UpdateTagPriority(ctx, userID, t.Name, t.Importance, t.Urgency); err == nil {
			result.ImportedTags++
		}
	}

	// Import settings
	if req.Settings != nil {
		if err := s.repo.SaveSettings(ctx, userID, *req.Settings); err == nil {
			result.SettingsApplied = true
		}
	}

	return nil, result, nil
}
