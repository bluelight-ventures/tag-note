package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/runminglu/tag-note/internal/apiclient"
)

// Server holds TagNote MCP handlers and shared API client state.
type Server struct {
	cfg    Config
	client *apiclient.Client
}

// New constructs an MCP server for TagNote.
func New(cfg Config, version string) (*mcp.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := apiclient.New(cfg.BaseURL, cfg.Token)
	client.UserAgent = "tagnote-mcp/" + version

	tagNote := &Server{
		cfg:    cfg,
		client: client,
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
