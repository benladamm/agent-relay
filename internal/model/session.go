package model

import "time"

// SessionStatus represents whether a session completed normally or was cut off by limits.
type SessionStatus string

const (
	StatusActive    SessionStatus = "ACTIVE"
	StatusCompleted SessionStatus = "COMPLETED"
	StatusRateLimit SessionStatus = "RATE_LIMIT"
	StatusError     SessionStatus = "ERROR"
)

// Session represents an AI coding agent conversation session.
type Session struct {
	ID           string        `json:"id"`
	AgentName    string        `json:"agent_name"`
	ProjectPath  string        `json:"project_path"`
	ProjectKey   string        `json:"project_key"`
	Transcript   string        `json:"transcript_path"`
	Status       SessionStatus `json:"status"`
	LimitReason  string        `json:"limit_reason,omitempty"`
	ResetsAt     time.Time     `json:"resets_at,omitempty"`
	LastModified time.Time     `json:"last_modified"`
	MessageCount int           `json:"message_count"`
	ToolCount    int           `json:"tool_count"`
	TotalBytes   int64         `json:"total_bytes"`
	Summary      string        `json:"summary,omitempty"`
}

// BundleManifest describes the contents of an exported bundle.
type BundleManifest struct {
	Version        string    `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	AgentName      string    `json:"agent_name"`
	SessionID      string    `json:"session_id"`
	SourceOS       string    `json:"source_os"`
	SourcePath     string    `json:"source_path"`
	HasGitDiff     bool      `json:"has_git_diff"`
	UntrackedFiles []string  `json:"untracked_files,omitempty"`
	Metadata       Session   `json:"metadata"`
}
