package model

import "time"

// AuditLog represents an entry in the audit log.
type AuditLog struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminOverview contains summary statistics for the admin dashboard.
type AdminOverview struct {
	TotalUsers  int    `json:"total_users"`
	TotalNotes  int    `json:"total_notes"`
	DAU         int    `json:"dau"`
	MAU         int    `json:"mau"`
	UptimeSec   int    `json:"uptime_sec"`
	Uptime      string `json:"uptime"`
	DBSizeBytes int    `json:"db_size_bytes"`
	DBSizeMB    string `json:"db_size_mb"`
	TrashCount  int    `json:"trash_count"`
	TagCount    int    `json:"tag_count"`
}

// AuditLogListResponse is the paginated response for listing audit logs.
type AuditLogListResponse struct {
	Logs   []AuditLog `json:"logs"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}
