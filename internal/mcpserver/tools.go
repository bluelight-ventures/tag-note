package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/runminglu/tag-note/internal/mcpoauth"
	"github.com/runminglu/tag-note/internal/model"
)

type emptyInput struct{}

type searchNotesInput struct {
	Tags           []string `json:"tags,omitempty" jsonschema:"only notes containing ALL of these tags are returned (AND match); omit to match any tag"`
	Query          string   `json:"query,omitempty" jsonschema:"full-text query matched against note content; combined with tags"`
	Limit          int      `json:"limit,omitempty" jsonschema:"maximum notes to return; capped to the server limit (default 50)"`
	Offset         int      `json:"offset,omitempty" jsonschema:"number of notes to skip, for paging through results"`
	Sort           string   `json:"sort,omitempty" jsonschema:"sort order: \"updated\" = most recently edited first; omit for newest-created first. Pinned notes always lead."`
	IncludeContent bool     `json:"include_content,omitempty" jsonschema:"include full note content (may be byte-truncated) instead of metadata only"`
}

type notesOutput struct {
	Notes            []NoteView `json:"notes"`
	Count            int        `json:"count"`
	ContentTruncated bool       `json:"content_truncated,omitempty"`
}

type getNoteInput struct {
	ID string `json:"id" jsonschema:"full note ID or unambiguous short ID"`
}

type noteOutput struct {
	Note NoteView `json:"note"`
}

type listTagsInput struct {
	Detailed bool `json:"detailed,omitempty" jsonschema:"return metadata per tag (status, note_count, importance, urgency) instead of names only"`
	Limit    int  `json:"limit,omitempty" jsonschema:"maximum tag names to return; applies only when detailed is false (detailed mode returns all tags)"`
}

type listTagsOutput struct {
	Tags         []string        `json:"tags,omitempty"`
	DetailedTags []model.TagInfo `json:"detailed_tags,omitempty"`
	Count        int             `json:"count"`
}

type autocompleteTagsInput struct {
	Prefix string `json:"prefix" jsonschema:"tag prefix to complete"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum tag names to return"`
}

type tagsOutput struct {
	Tags  []string `json:"tags"`
	Count int      `json:"count"`
}

type createNoteInput struct {
	Content string   `json:"content" jsonschema:"Markdown note content"`
	Tags    []string `json:"tags,omitempty" jsonschema:"tags to attach; unknown tags are created automatically as 'unreviewed'"`
	Pinned  bool     `json:"pinned,omitempty" jsonschema:"pin the note after creating it"`
}

type updateNoteInput struct {
	ID      string   `json:"id" jsonschema:"full note ID or unambiguous short ID"`
	Content *string  `json:"content,omitempty" jsonschema:"replacement Markdown content; omit to leave content unchanged"`
	Tags    []string `json:"tags,omitempty" jsonschema:"FULL replacement of the note's tags, not a merge: this list becomes the complete tag set. Omit to leave tags unchanged; pass [] to remove all tags. To add one tag, send the full desired list."`
}

type setPinnedInput struct {
	ID     string `json:"id" jsonschema:"full note ID or unambiguous short ID"`
	Pinned bool   `json:"pinned" jsonschema:"desired pinned state"`
}

type idOutput struct {
	ID string `json:"id"`
}

type tagNameInput struct {
	Name string `json:"name" jsonschema:"tag name"`
}

type renameTagInput struct {
	OldName string `json:"old_name" jsonschema:"existing tag name"`
	NewName string `json:"new_name" jsonschema:"new tag name; merges if it already exists"`
}

type updateTagPriorityInput struct {
	Name       string `json:"name" jsonschema:"tag name"`
	Importance int    `json:"importance" jsonschema:"importance from 0 to 100"`
	Urgency    int    `json:"urgency" jsonschema:"urgency from 0 to 100"`
}

type tagOutput struct {
	Name string `json:"name"`
}

type settingsOutput struct {
	Settings model.Settings `json:"settings"`
}

func (s *Server) registerTools(server *mcp.Server) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)}
	additive := &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: false, OpenWorldHint: boolPtr(false)}
	idempotent := &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)}
	destructive := &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: true, OpenWorldHint: boolPtr(false)}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_search_notes",
		Title:       "Search TagNote notes",
		Description: "Search notes by tag intersection and/or full-text query, returning structured JSON (id, tags, metadata, and optionally content). Paginated and capped — use for programmatic access, filtering, or iterating over results.",
		Annotations: readOnly,
	}, s.searchNotes)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_get_note",
		Title:       "Get TagNote note",
		Description: "Fetch one TagNote note by full ID or unambiguous short ID.",
		Annotations: readOnly,
	}, s.getNote)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_list_tags",
		Title:       "List TagNote tags",
		Description: "List tags. Returns names only by default; set detailed=true for metadata (status, note_count, importance, urgency). The limit applies only in names-only mode; detailed mode returns all tags.",
		Annotations: readOnly,
	}, s.listTags)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_autocomplete_tags",
		Title:       "Autocomplete TagNote tags",
		Description: "Return TagNote tag names matching a prefix.",
		Annotations: readOnly,
	}, s.autocompleteTags)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_list_trash",
		Title:       "List TagNote trash",
		Description: "List soft-deleted TagNote notes.",
		Annotations: readOnly,
	}, s.listTrash)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_get_settings",
		Title:       "Get TagNote settings",
		Description: "Fetch persisted TagNote user settings.",
		Annotations: readOnly,
	}, s.getSettings)

	if s.cfg.ReadOnly {
		return
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_create_note",
		Title:       "Create TagNote note",
		Description: "Create a Markdown note with optional tags and pin state. Unknown tags are created automatically.",
		Annotations: additive,
	}, s.createNote)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_update_note",
		Title:       "Update TagNote note",
		Description: "Update a note's content and/or tags. Provided fields overwrite (tags are replaced as a whole set, not merged); omitted fields are left unchanged.",
		Annotations: idempotent,
	}, s.updateNote)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_set_note_pinned",
		Title:       "Set TagNote pin state",
		Description: "Set a TagNote note's pinned state idempotently.",
		Annotations: idempotent,
	}, s.setNotePinned)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_restore_note",
		Title:       "Restore TagNote note",
		Description: "Restore a soft-deleted TagNote note.",
		Annotations: idempotent,
	}, s.restoreNote)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_approve_tag",
		Title:       "Approve TagNote tag",
		Description: "Mark a tag as approved (curated). New tags — including ones auto-created by notes — start as 'unreviewed'; approving signals it is intentional. Does not change which notes carry the tag.",
		Annotations: idempotent,
	}, s.approveTag)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_rename_tag",
		Title:       "Rename TagNote tag",
		Description: "Rename a TagNote tag, merging into the target if needed.",
		Annotations: idempotent,
	}, s.renameTag)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tagnote_update_tag_priority",
		Title:       "Update TagNote tag priority",
		Description: "Set a TagNote tag's importance and urgency from 0 to 100.",
		Annotations: idempotent,
	}, s.updateTagPriority)

	if s.cfg.AllowDelete {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "tagnote_delete_note",
			Title:       "Delete TagNote note",
			Description: "Move a TagNote note to trash. Permanent deletion is not exposed.",
			Annotations: destructive,
		}, s.deleteNote)
		mcp.AddTool(server, &mcp.Tool{
			Name:        "tagnote_delete_tag",
			Title:       "Delete TagNote tag",
			Description: "Remove a tag from all TagNote notes without deleting the notes.",
			Annotations: destructive,
		}, s.deleteTag)
	}
}

func (s *Server) searchNotes(ctx context.Context, req *mcp.CallToolRequest, in searchNotesInput) (*mcp.CallToolResult, notesOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeRead)
	if err != nil {
		return nil, notesOutput{}, err
	}
	notes, err := s.service.ReadNotes(ctx, userID, in.Tags, in.Query, s.cappedLimit(in.Limit), in.Offset, in.Sort)
	if err != nil {
		return nil, notesOutput{}, err
	}
	views, truncated := noteViews(notes, in.IncludeContent, s.cfg.MaxContentBytes)
	return nil, notesOutput{Notes: views, Count: len(views), ContentTruncated: truncated}, nil
}

func (s *Server) getNote(ctx context.Context, req *mcp.CallToolRequest, in getNoteInput) (*mcp.CallToolResult, noteOutput, error) {
	if in.ID == "" {
		return nil, noteOutput{}, fmt.Errorf("id is required")
	}
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeRead)
	if err != nil {
		return nil, noteOutput{}, err
	}
	note, err := s.service.GetNote(ctx, userID, in.ID)
	if err != nil {
		return nil, noteOutput{}, err
	}
	view := noteView(*note, true)
	view.Content, _ = capString(view.Content, s.cfg.MaxContentBytes)
	return nil, noteOutput{Note: view}, nil
}

func (s *Server) listTags(ctx context.Context, req *mcp.CallToolRequest, in listTagsInput) (*mcp.CallToolResult, listTagsOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeRead)
	if err != nil {
		return nil, listTagsOutput{}, err
	}
	if in.Detailed {
		tags, err := s.service.ListTagsDetailed(ctx, userID)
		if err != nil {
			return nil, listTagsOutput{}, err
		}
		return nil, listTagsOutput{DetailedTags: tags, Count: len(tags)}, nil
	}
	tags, err := s.service.ListTags(ctx, userID, in.Limit)
	if err != nil {
		return nil, listTagsOutput{}, err
	}
	return nil, listTagsOutput{Tags: tags, Count: len(tags)}, nil
}

func (s *Server) autocompleteTags(ctx context.Context, req *mcp.CallToolRequest, in autocompleteTagsInput) (*mcp.CallToolResult, tagsOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeRead)
	if err != nil {
		return nil, tagsOutput{}, err
	}
	tags, err := s.service.AutocompleteTags(ctx, userID, in.Prefix, s.cappedLimit(in.Limit))
	if err != nil {
		return nil, tagsOutput{}, err
	}
	return nil, tagsOutput{Tags: tags, Count: len(tags)}, nil
}

func (s *Server) listTrash(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, notesOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeRead)
	if err != nil {
		return nil, notesOutput{}, err
	}
	notes, err := s.service.ListTrashed(ctx, userID)
	if err != nil {
		return nil, notesOutput{}, err
	}
	views, truncated := noteViews(s.capNotes(notes), true, s.cfg.MaxContentBytes)
	return nil, notesOutput{Notes: views, Count: len(views), ContentTruncated: truncated}, nil
}

func (s *Server) getSettings(ctx context.Context, req *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, settingsOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeRead)
	if err != nil {
		return nil, settingsOutput{}, err
	}
	settings, err := s.service.GetSettings(ctx, userID)
	if err != nil {
		return nil, settingsOutput{}, err
	}
	return nil, settingsOutput{Settings: *settings}, nil
}

func (s *Server) createNote(ctx context.Context, req *mcp.CallToolRequest, in createNoteInput) (*mcp.CallToolResult, noteOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeWrite)
	if err != nil {
		return nil, noteOutput{}, err
	}
	resp, err := s.service.CreateNote(ctx, userID, model.CreateRequest{Content: in.Content, Tags: in.Tags})
	if err != nil {
		return nil, noteOutput{}, err
	}
	if in.Pinned {
		if err := s.service.TogglePin(ctx, userID, resp.ID); err != nil {
			return nil, noteOutput{}, err
		}
	}
	note, err := s.service.GetNote(ctx, userID, resp.ID)
	if err != nil {
		return nil, noteOutput{}, err
	}
	return nil, noteOutput{Note: noteView(*note, true)}, nil
}

func (s *Server) updateNote(ctx context.Context, req *mcp.CallToolRequest, in updateNoteInput) (*mcp.CallToolResult, noteOutput, error) {
	if in.ID == "" {
		return nil, noteOutput{}, fmt.Errorf("id is required")
	}
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeWrite)
	if err != nil {
		return nil, noteOutput{}, err
	}
	updateReq := model.UpdateRequest{Content: in.Content}
	if in.Tags != nil {
		updateReq.Tags = &in.Tags
	}
	note, err := s.service.UpdateNote(ctx, userID, in.ID, updateReq)
	if err != nil {
		return nil, noteOutput{}, err
	}
	return nil, noteOutput{Note: noteView(*note, true)}, nil
}

func (s *Server) setNotePinned(ctx context.Context, req *mcp.CallToolRequest, in setPinnedInput) (*mcp.CallToolResult, noteOutput, error) {
	if in.ID == "" {
		return nil, noteOutput{}, fmt.Errorf("id is required")
	}
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeWrite)
	if err != nil {
		return nil, noteOutput{}, err
	}
	note, err := s.service.GetNote(ctx, userID, in.ID)
	if err != nil {
		return nil, noteOutput{}, err
	}
	if note.Pinned != in.Pinned {
		if err := s.service.TogglePin(ctx, userID, in.ID); err != nil {
			return nil, noteOutput{}, err
		}
		note.Pinned = in.Pinned
	}
	return nil, noteOutput{Note: noteView(*note, true)}, nil
}

func (s *Server) restoreNote(ctx context.Context, req *mcp.CallToolRequest, in getNoteInput) (*mcp.CallToolResult, idOutput, error) {
	if in.ID == "" {
		return nil, idOutput{}, fmt.Errorf("id is required")
	}
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeWrite)
	if err != nil {
		return nil, idOutput{}, err
	}
	if err := s.service.RestoreNote(ctx, userID, in.ID); err != nil {
		return nil, idOutput{}, err
	}
	return nil, idOutput{ID: in.ID}, nil
}

func (s *Server) approveTag(ctx context.Context, req *mcp.CallToolRequest, in tagNameInput) (*mcp.CallToolResult, tagOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeWrite)
	if err != nil {
		return nil, tagOutput{}, err
	}
	if err := s.service.ApproveTag(ctx, userID, in.Name); err != nil {
		return nil, tagOutput{}, err
	}
	return nil, tagOutput{Name: in.Name}, nil
}

func (s *Server) renameTag(ctx context.Context, req *mcp.CallToolRequest, in renameTagInput) (*mcp.CallToolResult, tagOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeWrite)
	if err != nil {
		return nil, tagOutput{}, err
	}
	if err := s.service.RenameTag(ctx, userID, in.OldName, in.NewName); err != nil {
		return nil, tagOutput{}, err
	}
	return nil, tagOutput{Name: in.NewName}, nil
}

func (s *Server) updateTagPriority(ctx context.Context, req *mcp.CallToolRequest, in updateTagPriorityInput) (*mcp.CallToolResult, tagOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeWrite)
	if err != nil {
		return nil, tagOutput{}, err
	}
	if err := s.service.UpdateTagPriority(ctx, userID, in.Name, in.Importance, in.Urgency); err != nil {
		return nil, tagOutput{}, err
	}
	return nil, tagOutput{Name: in.Name}, nil
}

func (s *Server) deleteNote(ctx context.Context, req *mcp.CallToolRequest, in getNoteInput) (*mcp.CallToolResult, idOutput, error) {
	if in.ID == "" {
		return nil, idOutput{}, fmt.Errorf("id is required")
	}
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeDelete)
	if err != nil {
		return nil, idOutput{}, err
	}
	if err := s.service.DeleteNote(ctx, userID, in.ID); err != nil {
		return nil, idOutput{}, err
	}
	return nil, idOutput{ID: in.ID}, nil
}

func (s *Server) deleteTag(ctx context.Context, req *mcp.CallToolRequest, in tagNameInput) (*mcp.CallToolResult, tagOutput, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeDelete)
	if err != nil {
		return nil, tagOutput{}, err
	}
	if err := s.service.DeleteTag(ctx, userID, in.Name); err != nil {
		return nil, tagOutput{}, err
	}
	return nil, tagOutput{Name: in.Name}, nil
}
