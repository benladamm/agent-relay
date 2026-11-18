package antigravity

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benladamm/agent-relay/internal/model"
)

// AntigravityAdapter implements adapter.Adapter for Google Antigravity.
type AntigravityAdapter struct{}

func New() *AntigravityAdapter {
	return &AntigravityAdapter{}
}

func (a *AntigravityAdapter) ID() string {
	return "antigravity"
}

func (a *AntigravityAdapter) DisplayName() string {
	return "Antigravity"
}

func GetAntigravityHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity")
}

func (a *AntigravityAdapter) DetectHarness() (model.HarnessInfo, error) {
	home := GetAntigravityHome()
	info := model.HarnessInfo{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		HomePath:    home,
		Detected:    false,
	}

	if home == "" {
		return info, nil
	}

	brainDir := filepath.Join(home, "brain")
	if stat, err := os.Stat(brainDir); err == nil && stat.IsDir() {
		info.Detected = true
		projects, _ := a.DetectProjects()
		info.ProjectCount = len(projects)
		for _, p := range projects {
			info.SessionCount += p.SessionCount
		}
	}

	return info, nil
}

func (a *AntigravityAdapter) DetectProjects() ([]model.Project, error) {
	home := GetAntigravityHome()
	brainDir := filepath.Join(home, "brain")

	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, nil
	}

	// Map of ProjectKey -> Project
	projectMap := make(map[string]*model.Project)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		tPath := filepath.Join(brainDir, sessionID, ".system_generated", "logs", "transcript.jsonl")
		info, err := os.Stat(tPath)
		if err != nil {
			continue
		}

		workspacePath := a.extractWorkspace(tPath)
		if workspacePath == "" {
			workspacePath = "Default Workspace"
		}

		projKey := filepath.Clean(workspacePath)
		p, exists := projectMap[projKey]
		if !exists {
			dispName := filepath.Base(workspacePath)
			if dispName == "." || dispName == "/" || dispName == "\\" {
				dispName = "Antigravity Workspace"
			}
			p = &model.Project{
				HarnessID:    a.ID(),
				HarnessName:  a.DisplayName(),
				ProjectKey:   projKey,
				Path:         workspacePath,
				DisplayName:  dispName,
				LastModified: info.ModTime(),
			}
			projectMap[projKey] = p
		}

		p.SessionCount++
		if info.ModTime().After(p.LastModified) {
			p.LastModified = info.ModTime()
		}
	}

	var results []model.Project
	for _, p := range projectMap {
		results = append(results, *p)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].LastModified.After(results[j].LastModified)
	})

	return results, nil
}

func (a *AntigravityAdapter) extractWorkspace(transcriptPath string) string {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 512*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Command Working Directory:") {
			idx := strings.Index(line, "Command Working Directory:")
			sub := line[idx+len("Command Working Directory:"):]
			if end := strings.IndexAny(sub, "\r\n\\<\""); end != -1 {
				return strings.TrimSpace(sub[:end])
			}
			return strings.TrimSpace(sub)
		}
		if strings.Contains(line, "Active Workspaces:") {
			idx := strings.Index(line, "Active Workspaces:")
			sub := line[idx+len("Active Workspaces:"):]
			sub = strings.TrimPrefix(strings.TrimSpace(sub), "-")
			if end := strings.IndexAny(sub, "\r\n\\<\""); end != -1 {
				return strings.TrimSpace(sub[:end])
			}
			return strings.TrimSpace(sub)
		}
	}

	return ""
}

func (a *AntigravityAdapter) GetProjectSessions(projectKey string) ([]model.Session, error) {
	home := GetAntigravityHome()
	brainDir := filepath.Join(home, "brain")

	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, err
	}

	var results []model.Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		sessionDir := filepath.Join(brainDir, sessionID)
		tPath := filepath.Join(sessionDir, ".system_generated", "logs", "transcript.jsonl")
		info, err := os.Stat(tPath)
		if err != nil {
			continue
		}

		ws := a.extractWorkspace(tPath)
		if ws == "" {
			ws = "Default Workspace"
		}
		if filepath.Clean(ws) != projectKey {
			continue
		}

		session := a.parseSessionFile(tPath, sessionID, projectKey, ws, info)
		results = append(results, session)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].LastModified.After(results[j].LastModified)
	})

	return results, nil
}

func (a *AntigravityAdapter) parseSessionFile(filePath, sessionID, projectKey, workspace string, info os.FileInfo) model.Session {
	session := model.Session{
		ID:           sessionID,
		AgentName:    a.ID(),
		ProjectPath:  workspace,
		ProjectKey:   projectKey,
		Transcript:   filePath,
		Status:       model.StatusCompleted,
		LastModified: info.ModTime(),
		TotalBytes:   info.Size(),
	}

	f, err := os.Open(filePath)
	if err != nil {
		return session
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 512*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var lastUserMsg string
	for scanner.Scan() {
		line := scanner.Bytes()
		var obj struct {
			Type      string `json:"type"`
			Content   string `json:"content"`
			ToolCalls []any  `json:"tool_calls"`
			Status    string `json:"status"`
		}
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}

		if obj.Type == "USER_INPUT" {
			session.MessageCount++
			if strings.Contains(obj.Content, "<USER_REQUEST>") {
				start := strings.Index(obj.Content, "<USER_REQUEST>") + len("<USER_REQUEST>")
				end := strings.Index(obj.Content, "</USER_REQUEST>")
				if end > start {
					lastUserMsg = strings.TrimSpace(obj.Content[start:end])
				}
			}
		} else if obj.Type == "PLANNER_RESPONSE" {
			session.MessageCount++
			session.ToolCount += len(obj.ToolCalls)
		}

		if obj.Status == "ERROR" || strings.Contains(string(line), "quota") || strings.Contains(string(line), "rate_limit") {
			session.Status = model.StatusRateLimit
			session.LimitReason = "Limite détectée"
		}
	}

	if lastUserMsg != "" {
		if len(lastUserMsg) > 85 {
			lastUserMsg = lastUserMsg[:82] + "..."
		}
		session.Summary = lastUserMsg
	}

	return session
}

func (a *AntigravityAdapter) GetSessionFiles(session model.Session) (map[string]string, error) {
	files := make(map[string]string)
	files["transcript.jsonl"] = session.Transcript

	home := GetAntigravityHome()
	sessionBrain := filepath.Join(home, "brain", session.ID)

	if stat, err := os.Stat(sessionBrain); err == nil && stat.IsDir() {
		filepath.Walk(sessionBrain, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				rel, _ := filepath.Rel(sessionBrain, p)
				files[filepath.Join("brain", rel)] = p
			}
			return nil
		})
	}

	return files, nil
}

func (a *AntigravityAdapter) InstallSession(manifest model.BundleManifest, files map[string][]byte, targetProjectPath string) error {
	home := GetAntigravityHome()
	brainDir := filepath.Join(home, "brain", manifest.SessionID)

	for relPath, content := range files {
		var destPath string
		if strings.HasPrefix(relPath, "brain") {
			sub := strings.TrimPrefix(relPath, "brain")
			sub = strings.TrimPrefix(sub, string(filepath.Separator))
			destPath = filepath.Join(brainDir, sub)
		} else {
			destPath = filepath.Join(brainDir, ".system_generated", "logs", "transcript.jsonl")
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return err
		}
	}

	return nil
}
