package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerPrompts(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "capture_note",
		Title:       "Capture TagNote note",
		Description: "Turn source text into a concise TagNote note with suggested tags.",
		Arguments: []*mcp.PromptArgument{
			{Name: "source_text", Description: "Text to capture", Required: true},
			{Name: "suggested_tags", Description: "Comma-separated suggested tags"},
		},
	}, s.getPrompt)
	server.AddPrompt(&mcp.Prompt{
		Name:        "summarize_tag",
		Title:       "Summarize TagNote tag",
		Description: "Summarize notes for a tag and cite note IDs.",
		Arguments: []*mcp.PromptArgument{
			{Name: "tag", Description: "Tag to summarize", Required: true},
			{Name: "query", Description: "Optional full-text query"},
		},
	}, s.getPrompt)
	server.AddPrompt(&mcp.Prompt{
		Name:        "organize_tags",
		Title:       "Organize TagNote tags",
		Description: "Review tags and propose renames, merges, approvals, or priority updates.",
		Arguments: []*mcp.PromptArgument{
			{Name: "query", Description: "Optional theme or area to focus on"},
		},
	}, s.getPrompt)
	server.AddPrompt(&mcp.Prompt{
		Name:        "weekly_review",
		Title:       "TagNote weekly review",
		Description: "Build a weekly review from selected tags or search terms.",
		Arguments: []*mcp.PromptArgument{
			{Name: "tags", Description: "Comma-separated tags to review"},
			{Name: "query", Description: "Optional full-text query"},
		},
	}, s.getPrompt)
}

func (s *Server) getPrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments
	var text string
	switch req.Params.Name {
	case "capture_note":
		text = fmt.Sprintf(`Create a concise TagNote note from this source text.

Source text:
%s

Suggested tags: %s

First decide whether the content is worth saving. If it is, produce Markdown note content and a normalized lowercase tag list. Then use tagnote_create_note only after the user confirms, unless the user already explicitly asked you to save it.`, args["source_text"], args["suggested_tags"])
	case "summarize_tag":
		text = fmt.Sprintf(`Use TagNote tools to search notes with tag %q and optional query %q.

Summarize the recurring themes, open loops, and useful next actions. Cite note short IDs or full IDs for every concrete claim based on note content.`, args["tag"], args["query"])
	case "organize_tags":
		text = fmt.Sprintf(`Use TagNote tag and note search tools to review the tag system%s.

Identify duplicate, vague, stale, or unapproved tags. Propose changes first. Only use rename, approve, delete, or priority tools after the user confirms the specific changes.`, optionalFocus(args["query"]))
	case "weekly_review":
		text = fmt.Sprintf(`Use TagNote search and stream tools to build a weekly review.

Tags: %s
Query: %s

Find highlights, decisions, unresolved tasks, and themes. Cite note IDs. Do not create or update notes unless the user asks for a saved review note.`, args["tags"], args["query"])
	default:
		return nil, fmt.Errorf("unknown prompt %q", req.Params.Name)
	}
	return &mcp.GetPromptResult{
		Description: "TagNote workflow prompt",
		Messages: []*mcp.PromptMessage{{
			Role:    mcp.Role("user"),
			Content: &mcp.TextContent{Text: text},
		}},
	}, nil
}

func optionalFocus(query string) string {
	if query == "" {
		return ""
	}
	return " with focus: " + query
}
