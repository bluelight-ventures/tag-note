package mcpserver

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultMaxNotes        = 50
	defaultMaxContentBytes = 200000
)

// Config controls the TagNote MCP server.
type Config struct {
	Addr            string
	DBPath          string
	UploadsDir      string
	PublicURL       string
	ResourcePath    string
	ReadOnly        bool
	AllowDelete     bool
	MaxNotes        int
	MaxContentBytes int
}

// ConfigFromEnv reads MCP configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		Addr:            envString("TAGNOTE_MCP_ADDR", ":3001"),
		DBPath:          envString("TAGNOTE_DB", "/data/tagnote.db"),
		UploadsDir:      envString("TAGNOTE_UPLOADS", "/data/uploads"),
		PublicURL:       envString("TAGNOTE_MCP_PUBLIC_URL", "http://localhost:3779"),
		ResourcePath:    envString("TAGNOTE_MCP_RESOURCE_PATH", "/mcp"),
		ReadOnly:        envBool("TAGNOTE_MCP_READ_ONLY", false),
		AllowDelete:     envBool("TAGNOTE_MCP_ALLOW_DELETE", false),
		MaxNotes:        envInt("TAGNOTE_MCP_MAX_NOTES", defaultMaxNotes),
		MaxContentBytes: envInt("TAGNOTE_MCP_MAX_CONTENT_BYTES", defaultMaxContentBytes),
	}
}

// Validate checks required settings and fills conservative defaults.
func (c *Config) Validate() error {
	c.Addr = strings.TrimSpace(c.Addr)
	if c.Addr == "" {
		c.Addr = ":3001"
	}
	c.DBPath = strings.TrimSpace(c.DBPath)
	if c.DBPath == "" {
		return fmt.Errorf("TAGNOTE_DB is required")
	}
	c.UploadsDir = strings.TrimSpace(c.UploadsDir)
	if c.UploadsDir == "" {
		c.UploadsDir = "/data/uploads"
	}
	c.PublicURL = strings.TrimRight(strings.TrimSpace(c.PublicURL), "/")
	if c.PublicURL == "" {
		return fmt.Errorf("TAGNOTE_MCP_PUBLIC_URL is required")
	}
	c.ResourcePath = "/" + strings.Trim(strings.TrimSpace(c.ResourcePath), "/")
	if c.ResourcePath == "/" {
		c.ResourcePath = "/mcp"
	}
	if c.MaxNotes <= 0 {
		c.MaxNotes = defaultMaxNotes
	}
	if c.MaxContentBytes <= 0 {
		c.MaxContentBytes = defaultMaxContentBytes
	}
	return nil
}

func (c Config) ResourceURL() string {
	return strings.TrimRight(c.PublicURL, "/") + c.ResourcePath
}

func (c Config) ResourceMetadataURL() string {
	return strings.TrimRight(c.PublicURL, "/") + "/.well-known/oauth-protected-resource" + c.ResourcePath
}

func envString(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
