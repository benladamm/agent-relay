package windsurf

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/benladamm/agent-relay/internal/model"
)

// WindsurfAdapter implements adapter.Adapter for Windsurf (Cascade).
type WindsurfAdapter struct{}

func New() *WindsurfAdapter {
	return &WindsurfAdapter{}
}

func (a *WindsurfAdapter) ID() string {
	return "windsurf"
}

func (a *WindsurfAdapter) DisplayName() string {
	return "Windsurf"
}

func GetWindsurfStorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Windsurf", "User", "workspaceStorage")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Windsurf", "User", "workspaceStorage")
	default:
		return filepath.Join(home, ".config", "Windsurf", "User", "workspaceStorage")
	}
}

func (a *WindsurfAdapter) DetectHarness() (model.HarnessInfo, error) {
	dir := GetWindsurfStorageDir()
	info := model.HarnessInfo{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		HomePath:    dir,
		Detected:    false,
	}

	if dir == "" {
		return info, nil
	}

	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		info.Detected = true
		projects, _ := a.DetectProjects()
		info.ProjectCount = len(projects)
		for _, p := range projects {
			info.SessionCount += p.SessionCount
		}
	}

	return info, nil
}

type workspaceMeta struct {
	Folder string `json:"folder"`
}

func (a *WindsurfAdapter) DetectProjects() ([]model.Project, error) {
	dir := GetWindsurfStorageDir()
	if dir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var projects []model.Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wsDir := filepath.Join(dir, entry.Name())
		wsFile := filepath.Join(wsDir, "workspace.json")
		data, err := os.ReadFile(wsFile)
		if err != nil {
			continue
		}

		var meta workspaceMeta
		if err := json.Unmarshal(data, &meta); err != nil || meta.Folder == "" {
			continue
		}

		folderPath := meta.Folder
		if strings.HasPrefix(folderPath, "file://") {
			if u, err := url.Parse(folderPath); err == nil {
				folderPath = filepath.FromSlash(u.Path)
				if runtime.GOOS == "windows" {
					folderPath = strings.TrimPrefix(folderPath, `\`)
					folderPath = strings.TrimPrefix(folderPath, `/`)
				}
			}
		}

		info, _ := entry.Info()
		modTime := info.ModTime()

		sessionCount := 1
		stateDB := filepath.Join(wsDir, "state.vscdb")
		if fi, err := os.Stat(stateDB); err == nil && fi.ModTime().After(modTime) {
			modTime = fi.ModTime()
		}

		projects = append(projects, model.Project{
			HarnessID:    a.ID(),
			HarnessName:  a.DisplayName(),
			ProjectKey:   entry.Name(),
			Path:         folderPath,
			DisplayName:  filepath.Base(folderPath),
			LastModified: modTime,
			SessionCount: sessionCount,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

func (a *WindsurfAdapter) GetProjectSessions(projectKey string) ([]model.Session, error) {
	dir := GetWindsurfStorageDir()
	wsDir := filepath.Join(dir, projectKey)

	stateDB := filepath.Join(wsDir, "state.vscdb")
	info, err := os.Stat(stateDB)
	if err != nil {
		return nil, err
	}

	session := model.Session{
		ID:           projectKey,
		AgentName:    a.ID(),
		ProjectKey:   projectKey,
		Transcript:   stateDB,
		Status:       model.StatusCompleted,
		LastModified: info.ModTime(),
		TotalBytes:   info.Size(),
		Summary:      "Windsurf Cascade Session State",
	}

	return []model.Session{session}, nil
}

func (a *WindsurfAdapter) GetSessionFiles(session model.Session) (map[string]string, error) {
	dir := GetWindsurfStorageDir()
	wsDir := filepath.Join(dir, session.ProjectKey)

	files := make(map[string]string)
	filepath.Walk(wsDir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			rel, _ := filepath.Rel(wsDir, p)
			files[filepath.Join("workspace", rel)] = p
		}
		return nil
	})

	return files, nil
}

func (a *WindsurfAdapter) InstallSession(manifest model.BundleManifest, files map[string][]byte, targetProjectPath string) error {
	dir := GetWindsurfStorageDir()
	destDir := filepath.Join(dir, manifest.SessionID)

	for relPath, content := range files {
		sub := strings.TrimPrefix(relPath, "workspace")
		sub = strings.TrimPrefix(sub, string(filepath.Separator))
		dest := filepath.Join(destDir, sub)

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return err
		}
	}

	return nil
}
