package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/runminglu/tag-note/internal/mcpoauth"
)

func (s *Server) registerResources(server *mcp.Server) {
	server.AddResource(&mcp.Resource{
		URI:         "tagnote://tags",
		Name:        "tagnote_tags",
		Title:       "TagNote tags",
		Description: "Detailed TagNote tag index with counts and priorities.",
		MIMEType:    "application/json",
	}, s.readResource)
	server.AddResource(&mcp.Resource{
		URI:         "tagnote://settings",
		Name:        "tagnote_settings",
		Title:       "TagNote settings",
		Description: "Current TagNote user settings.",
		MIMEType:    "application/json",
	}, s.readResource)
	server.AddResource(&mcp.Resource{
		URI:         "tagnote://trash",
		Name:        "tagnote_trash",
		Title:       "TagNote trash",
		Description: "Soft-deleted TagNote notes, capped by MCP server policy.",
		MIMEType:    "application/json",
	}, s.readResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "tagnote://notes/{id}",
		Name:        "tagnote_note",
		Title:       "TagNote note",
		Description: "A single TagNote note as JSON.",
		MIMEType:    "application/json",
	}, s.readResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "tagnote://notes/{id}.md",
		Name:        "tagnote_note_markdown",
		Title:       "TagNote note Markdown",
		Description: "A single TagNote note as Markdown.",
		MIMEType:    "text/markdown",
	}, s.readResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "tagnote://search{?tag,q,limit,sort}",
		Name:        "tagnote_search",
		Title:       "TagNote search",
		Description: "Search TagNote notes as JSON.",
		MIMEType:    "application/json",
	}, s.readResource)
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "tagnote://stream{?tag,q}",
		Name:        "tagnote_stream",
		Title:       "TagNote Markdown stream",
		Description: "Matching TagNote notes rendered as Markdown.",
		MIMEType:    "text/markdown",
	}, s.readResource)
}

func (s *Server) readResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	userID, err := userIDFromToken(req.Extra.TokenInfo, mcpoauth.ScopeRead)
	if err != nil {
		return nil, err
	}
	uri := req.Params.URI
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "tagnote" {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	switch parsed.Host {
	case "tags":
		tags, err := s.service.ListTagsDetailed(ctx, userID)
		if err != nil {
			return nil, err
		}
		return jsonResource(uri, tags)
	case "settings":
		settings, err := s.service.GetSettings(ctx, userID)
		if err != nil {
			return nil, err
		}
		return jsonResource(uri, settings)
	case "trash":
		notes, err := s.service.ListTrashed(ctx, userID)
		if err != nil {
			return nil, err
		}
		if len(notes) > s.cfg.MaxNotes {
			notes = notes[:s.cfg.MaxNotes]
		}
		views, _ := noteViews(notes, true, s.cfg.MaxContentBytes)
		return jsonResource(uri, notesOutput{Notes: views, Count: len(views)})
	case "notes":
		return s.readNoteResource(ctx, userID, uri, strings.TrimPrefix(parsed.Path, "/"))
	case "search":
		return s.readSearchResource(ctx, userID, uri, parsed.Query())
	case "stream":
		return s.readStreamResource(ctx, userID, uri, parsed.Query())
	default:
		return nil, mcp.ResourceNotFoundError(uri)
	}
}

func (s *Server) readNoteResource(ctx context.Context, userID, uri, idPath string) (*mcp.ReadResourceResult, error) {
	asMarkdown := strings.HasSuffix(idPath, ".md")
	id := strings.TrimSuffix(idPath, ".md")
	if id == "" {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	note, err := s.service.GetNote(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if asMarkdown {
		content, _ := capString(note.Content, s.cfg.MaxContentBytes)
		return textResource(uri, "text/markdown", content), nil
	}
	view := noteView(*note, true)
	view.Content, _ = capString(view.Content, s.cfg.MaxContentBytes)
	return jsonResource(uri, noteOutput{Note: view})
}

func (s *Server) readSearchResource(ctx context.Context, userID, uri string, query url.Values) (*mcp.ReadResourceResult, error) {
	limit := s.cappedLimit(parsePositiveInt(query.Get("limit")))
	notes, err := s.service.ReadNotes(ctx, userID, query["tag"], query.Get("q"), limit, 0, query.Get("sort"))
	if err != nil {
		return nil, err
	}
	views, truncated := noteViews(notes, false, s.cfg.MaxContentBytes)
	return jsonResource(uri, notesOutput{Notes: views, Count: len(views), ContentTruncated: truncated})
}

func (s *Server) readStreamResource(ctx context.Context, userID, uri string, query url.Values) (*mcp.ReadResourceResult, error) {
	md, err := s.service.RenderStream(ctx, userID, query["tag"], query.Get("q"))
	if err != nil {
		return nil, err
	}
	md, _ = capString(md, s.cfg.MaxContentBytes)
	return textResource(uri, "text/markdown", md), nil
}

func jsonResource(uri string, value any) (*mcp.ReadResourceResult, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal resource: %w", err)
	}
	return textResource(uri, "application/json", string(b)), nil
}

func textResource(uri, mimeType, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: mimeType,
			Text:     text,
		}},
	}
}

func parsePositiveInt(raw string) int {
	if raw == "" {
		return 0
	}
	var n int
	_, _ = fmt.Sscan(raw, &n)
	if n < 0 {
		return 0
	}
	return n
}
