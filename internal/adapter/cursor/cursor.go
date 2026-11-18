package cursor

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

// CursorAdapter implements adapter.Adapter for Cursor IDE.
type CursorAdapter struct{}

func New() *CursorAdapter {
	return &CursorAdapter{}
}

func (a *CursorAdapter) ID() string {
	return "cursor"
}

func (a *CursorAdapter) DisplayName() string {
	return "Cursor"
}

func GetCursorStorageDir() string {
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
		return filepath.Join(appData, "Cursor", "User", "workspaceStorage")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User", "workspaceStorage")
	default:
		// Linux & BSD
		return filepath.Join(home, ".config", "Cursor", "User", "workspaceStorage")
	}
}

func (a *CursorAdapter) DetectHarness() (model.HarnessInfo, error) {
	dir := GetCursorStorageDir()
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

func (a *CursorAdapter) DetectProjects() ([]model.Project, error) {
	dir := GetCursorStorageDir()
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

func (a *CursorAdapter) GetProjectSessions(projectKey string) ([]model.Session, error) {
	dir := GetCursorStorageDir()
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
		Summary:      "Cursor Composer / Chat Workspace State",
	}

	return []model.Session{session}, nil
}

func (a *CursorAdapter) GetSessionFiles(session model.Session) (map[string]string, error) {
	dir := GetCursorStorageDir()
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

func (a *CursorAdapter) InstallSession(manifest model.BundleManifest, files map[string][]byte, targetProjectPath string) error {
	dir := GetCursorStorageDir()
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
