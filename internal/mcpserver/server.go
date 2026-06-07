package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/runminglu/tag-note/internal/service"
)

// Server holds TagNote MCP handlers and shared service state.
type Server struct {
	cfg     Config
	service *service.Service
}

// New constructs an MCP server for TagNote.
func New(cfg Config, svc *service.Service, version string) (*mcp.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	tagNote := &Server{
		cfg:     cfg,
		service: svc,
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "tagnote",
		Version: version,
	}, nil)
	tagNote.registerTools(server)
	tagNote.registerResources(server)
	tagNote.registerPrompts(server)
	return server, nil
}

func boolPtr(v bool) *bool {
	return &v
}
