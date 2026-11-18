package adapter

import "github.com/benladamm/agent-relay/internal/model"

// Adapter defines the operations needed to interact with an AI coding harness.
type Adapter interface {
	ID() string
	DisplayName() string
	DetectHarness() (model.HarnessInfo, error)
	DetectProjects() ([]model.Project, error)
	GetProjectSessions(projectKey string) ([]model.Session, error)
	GetSessionFiles(session model.Session) (map[string]string, error)
	InstallSession(manifest model.BundleManifest, files map[string][]byte, targetProjectPath string) error
}
