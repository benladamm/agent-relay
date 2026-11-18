package model

import "time"

// HarnessInfo represents a detected AI agent environment on the system.
type HarnessInfo struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	HomePath     string `json:"home_path"`
	Detected     bool   `json:"detected"`
	ProjectCount int    `json:"project_count"`
	SessionCount int    `json:"session_count"`
}

// Project represents a project directory associated with one or more sessions.
type Project struct {
	HarnessID    string    `json:"harness_id"`
	HarnessName  string    `json:"harness_name"`
	ProjectKey   string    `json:"project_key"`
	Path         string    `json:"path"`
	DisplayName  string    `json:"display_name"`
	LastModified time.Time `json:"last_modified"`
	SessionCount int       `json:"session_count"`
	QuotaHits    int       `json:"quota_hits"`
}
