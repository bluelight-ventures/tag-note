package admin

import (
	"os"
	"strings"
)

// AdminConfig holds admin-specific configuration.
type AdminConfig struct {
	AdminEmail string
}

// LoadConfig reads admin configuration from environment variables.
// Returns an empty config if ADMIN_EMAIL is not set (admin features disabled).
func LoadConfig() AdminConfig {
	return AdminConfig{
		AdminEmail: strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))),
	}
}

// IsEnabled returns true if admin features are enabled (ADMIN_EMAIL is set).
func (c AdminConfig) IsEnabled() bool {
	return c.AdminEmail != ""
}
