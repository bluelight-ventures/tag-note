package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/runminglu/tag-note/internal/mcpserver"
)

// Build-time variables set via -ldflags.
var (
	Version = "dev"
)

func main() {
	log.SetOutput(os.Stderr)

	cfg := mcpserver.ConfigFromEnv()
	flag.StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "TagNote base URL")
	flag.BoolVar(&cfg.ReadOnly, "read-only", cfg.ReadOnly, "register only read tools")
	flag.BoolVar(&cfg.AllowDelete, "allow-delete", cfg.AllowDelete, "register soft-delete tools")
	flag.IntVar(&cfg.MaxNotes, "max-notes", cfg.MaxNotes, "maximum notes returned by one MCP call")
	flag.IntVar(&cfg.MaxContentBytes, "max-content-bytes", cfg.MaxContentBytes, "maximum note content bytes returned by one MCP call")
	flag.Parse()

	server, err := mcpserver.New(cfg, Version)
	if err != nil {
		log.Fatalf("configure tagnote-mcp: %v", err)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run tagnote-mcp: %v", err)
	}
}
