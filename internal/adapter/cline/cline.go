package cline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/benladamm/agent-relay/internal/model"
)

// ClineAdapter implements adapter.Adapter for Cline & Roo Code.
type ClineAdapter struct{}

func New() *ClineAdapter {
	return &ClineAdapter{}
}

func (a *ClineAdapter) ID() string {
	return "cline"
}

func (a *ClineAdapter) DisplayName() string {
	return "Cline / Roo Code"
}

func GetClineTaskDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var baseDir string
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		baseDir = filepath.Join(appData, "Code", "User", "globalStorage")
	case "darwin":
		baseDir = filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage")
	default:
		baseDir = filepath.Join(home, ".config", "Code", "User", "globalStorage")
	}

	return []string{
		filepath.Join(baseDir, "saoudrizwan.claude-dev", "tasks"),
		filepath.Join(baseDir, "rooveterinaryinc.roo-cline", "tasks"),
	}
}

func (a *ClineAdapter) DetectHarness() (model.HarnessInfo, error) {
	dirs := GetClineTaskDirs()
	info := model.HarnessInfo{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		Detected:    false,
	}

	for _, d := range dirs {
		if stat, err := os.Stat(d); err == nil && stat.IsDir() {
			info.Detected = true
			info.HomePath = d
			break
		}
	}

	if info.Detected {
		projects, _ := a.DetectProjects()
		info.ProjectCount = len(projects)
		for _, p := range projects {
			info.SessionCount += p.SessionCount
		}
	}

	return info, nil
}

func (a *ClineAdapter) DetectProjects() ([]model.Project, error) {
	dirs := GetClineTaskDirs()
	var projects []model.Project

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			taskDir := filepath.Join(dir, e.Name())
			histFile := filepath.Join(taskDir, "api_conversation_history.json")
			fi, err := os.Stat(histFile)
			if err != nil {
				continue
			}

			projects = append(projects, model.Project{
				HarnessID:    a.ID(),
				HarnessName:  a.DisplayName(),
				ProjectKey:   e.Name(),
				Path:         taskDir,
				DisplayName:  "Task " + e.Name()[:8],
				LastModified: fi.ModTime(),
				SessionCount: 1,
			})
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

func (a *ClineAdapter) GetProjectSessions(projectKey string) ([]model.Session, error) {
	dirs := GetClineTaskDirs()
	for _, dir := range dirs {
		taskDir := filepath.Join(dir, projectKey)
		histFile := filepath.Join(taskDir, "api_conversation_history.json")
		fi, err := os.Stat(histFile)
		if err != nil {
			continue
		}

		session := model.Session{
			ID:           projectKey,
			AgentName:    a.ID(),
			ProjectKey:   projectKey,
			Transcript:   histFile,
			Status:       model.StatusCompleted,
			LastModified: fi.ModTime(),
			TotalBytes:   fi.Size(),
			Summary:      extractClineSummary(histFile),
		}
		return []model.Session{session}, nil
	}
	return nil, nil
}

func extractClineSummary(histPath string) string {
	data, err := os.ReadFile(histPath)
	if err != nil {
		return "Cline Autonomous Task"
	}

	var msgs []struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	if err := json.Unmarshal(data, &msgs); err == nil && len(msgs) > 0 {
		for _, m := range msgs {
			if m.Role == "user" {
				if s, ok := m.Content.(string); ok && len(s) > 0 {
					clean := strings.TrimSpace(s)
					if len(clean) > 85 {
						clean = clean[:82] + "..."
					}
					return clean
				}
			}
		}
	}
	return "Cline Task Session"
}

func (a *ClineAdapter) GetSessionFiles(session model.Session) (map[string]string, error) {
	files := make(map[string]string)
	dirs := GetClineTaskDirs()

	for _, dir := range dirs {
		taskDir := filepath.Join(dir, session.ProjectKey)
		if stat, err := os.Stat(taskDir); err == nil && stat.IsDir() {
			filepath.Walk(taskDir, func(p string, fi os.FileInfo, err error) error {
				if err == nil && !fi.IsDir() {
					rel, _ := filepath.Rel(taskDir, p)
					files[filepath.Join("task", rel)] = p
				}
				return nil
			})
			break
		}
	}

	return files, nil
}

func (a *ClineAdapter) InstallSession(manifest model.BundleManifest, files map[string][]byte, targetProjectPath string) error {
	dirs := GetClineTaskDirs()
	if len(dirs) == 0 {
		return nil
	}
	targetDir := filepath.Join(dirs[0], manifest.SessionID)

	for relPath, content := range files {
		sub := strings.TrimPrefix(relPath, "task")
		sub = strings.TrimPrefix(sub, string(filepath.Separator))
		dest := filepath.Join(targetDir, sub)

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return err
		}
	}

	return nil
}
