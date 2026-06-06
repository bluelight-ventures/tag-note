package mcpserver

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/runminglu/tag-note/internal/apiclient"
)

const (
	defaultMaxNotes        = 50
	defaultMaxContentBytes = 200000
)

// Config controls the TagNote MCP server.
type Config struct {
	BaseURL         string
	Token           string
	ReadOnly        bool
	AllowDelete     bool
	MaxNotes        int
	MaxContentBytes int
}

// ConfigFromEnv reads MCP configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		BaseURL:         envString("TAGNOTE_URL", apiclient.BaseURL()),
		Token:           os.Getenv("TAGNOTE_TOKEN"),
		ReadOnly:        envBool("TAGNOTE_MCP_READ_ONLY", false),
		AllowDelete:     envBool("TAGNOTE_MCP_ALLOW_DELETE", false),
		MaxNotes:        envInt("TAGNOTE_MCP_MAX_NOTES", defaultMaxNotes),
		MaxContentBytes: envInt("TAGNOTE_MCP_MAX_CONTENT_BYTES", defaultMaxContentBytes),
	}
}

// Validate checks required settings and fills conservative defaults.
func (c *Config) Validate() error {
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if c.BaseURL == "" {
		c.BaseURL = apiclient.BaseURL()
	}
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("TAGNOTE_TOKEN is required")
	}
	if c.MaxNotes <= 0 {
		c.MaxNotes = defaultMaxNotes
	}
	if c.MaxContentBytes <= 0 {
		c.MaxContentBytes = defaultMaxContentBytes
	}
	return nil
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
