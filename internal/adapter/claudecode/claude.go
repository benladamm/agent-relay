package claudecode

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/benladamm/agent-relay/internal/model"
)

// ClaudeAdapter implements adapter.Adapter for Claude Code.
type ClaudeAdapter struct{}

func New() *ClaudeAdapter {
	return &ClaudeAdapter{}
}

func (a *ClaudeAdapter) ID() string {
	return "claude-code"
}

func (a *ClaudeAdapter) DisplayName() string {
	return "Claude Code"
}

func GetClaudeHome() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

func (a *ClaudeAdapter) DetectHarness() (model.HarnessInfo, error) {
	home := GetClaudeHome()
	info := model.HarnessInfo{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		HomePath:    home,
		Detected:    false,
	}

	if home == "" {
		return info, nil
	}

	if stat, err := os.Stat(home); err == nil && stat.IsDir() {
		info.Detected = true
		projects, _ := a.DetectProjects()
		info.ProjectCount = len(projects)
		for _, p := range projects {
			info.SessionCount += p.SessionCount
		}
	}

	return info, nil
}

func (a *ClaudeAdapter) DetectProjects() ([]model.Project, error) {
	claudeHome := GetClaudeHome()
	if claudeHome == "" {
		return nil, nil
	}

	projectsDir := filepath.Join(claudeHome, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, nil
	}

	var projects []model.Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projDir := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}

		sessionCount := 0
		quotaHits := 0
		var latestMod time.Time
		var samplePath string

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			sessionCount++
			if info, err := f.Info(); err == nil {
				if info.ModTime().After(latestMod) {
					latestMod = info.ModTime()
				}
			}

			// Inspect header or last line of first session to find path
			if samplePath == "" {
				samplePath = extractCwdFromTranscript(filepath.Join(projDir, f.Name()))
			}
		}

		if sessionCount == 0 {
			continue
		}

		displayName := entry.Name()
		if samplePath != "" {
			displayName = filepath.Base(samplePath)
		} else {
			parts := strings.Split(entry.Name(), "-")
			if len(parts) > 0 {
				displayName = parts[len(parts)-1]
			}
		}

		projects = append(projects, model.Project{
			HarnessID:    a.ID(),
			HarnessName:  a.DisplayName(),
			ProjectKey:   entry.Name(),
			Path:         samplePath,
			DisplayName:  displayName,
			LastModified: latestMod,
			SessionCount: sessionCount,
			QuotaHits:    quotaHits,
		})
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

func extractCwdFromTranscript(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 512*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var obj struct {
			Cwd string `json:"cwd"`
		}
		if err := json.Unmarshal(line, &obj); err == nil && obj.Cwd != "" {
			return obj.Cwd
		}
	}
	return ""
}

func (a *ClaudeAdapter) GetProjectSessions(projectKey string) ([]model.Session, error) {
	claudeHome := GetClaudeHome()
	projDir := filepath.Join(claudeHome, "projects", projectKey)

	files, err := os.ReadDir(projDir)
	if err != nil {
		return nil, err
	}

	var results []model.Session
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(f.Name(), ".jsonl")
		filePath := filepath.Join(projDir, f.Name())
		info, err := f.Info()
		if err != nil {
			continue
		}

		session := a.parseSessionFile(filePath, sessionID, projectKey, info)
		results = append(results, session)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].LastModified.After(results[j].LastModified)
	})

	return results, nil
}

type rawJsonLine struct {
	Type          string          `json:"type"`
	APIError      string          `json:"apiError"`
	APIErrorType  string          `json:"apiErrorType"`
	APIErrorParam json.RawMessage `json:"apiErrorParams"`
	Message       struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type rateLimitWrapper struct {
	RateLimitInfo struct {
		Status    string `json:"status"`
		ResetsAt  int64  `json:"resetsAt"`
		LimitType string `json:"rateLimitType"`
	} `json:"rate_limit_info"`
}

func (a *ClaudeAdapter) parseSessionFile(filePath, sessionID, projectKey string, info os.FileInfo) model.Session {
	session := model.Session{
		ID:           sessionID,
		AgentName:    a.ID(),
		ProjectKey:   projectKey,
		Transcript:   filePath,
		Status:       model.StatusCompleted,
		LastModified: info.ModTime(),
		TotalBytes:   info.Size(),
	}

	file, err := os.Open(filePath)
	if err != nil {
		return session
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var lastText string
	for scanner.Scan() {
		line := scanner.Bytes()
		var item rawJsonLine
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}

		if item.Type == "user" || item.Type == "assistant" {
			session.MessageCount++
		}

		if strings.Contains(string(line), `"type":"tool_use"`) {
			session.ToolCount++
		}

		if item.APIError == "usage_limit_reached" ||
			item.APIErrorType == "rate_limit_error" ||
			strings.Contains(string(line), "You've hit your") ||
			strings.Contains(string(line), "limit · resets") {
			session.Status = model.StatusRateLimit
			session.LimitReason = "Limite de quota atteinte"

			var rlw rateLimitWrapper
			if err := json.Unmarshal(item.APIErrorParam, &rlw); err == nil && rlw.RateLimitInfo.ResetsAt > 0 {
				session.ResetsAt = time.Unix(rlw.RateLimitInfo.ResetsAt, 0)
			}
		}

		if item.Type == "user" && len(item.Message.Content) > 0 {
			var text string
			if err := json.Unmarshal(item.Message.Content, &text); err == nil {
				if !strings.HasPrefix(text, "<task-notification>") {
					lastText = text
				}
			} else {
				var parts []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}
				if err := json.Unmarshal(item.Message.Content, &parts); err == nil {
					for _, p := range parts {
						if p.Type == "text" && !strings.HasPrefix(p.Text, "<task-notification>") {
							lastText = p.Text
						}
					}
				}
			}
		}
	}

	if lastText != "" {
		clean := strings.TrimSpace(lastText)
		if len(clean) > 85 {
			clean = clean[:82] + "..."
		}
		session.Summary = clean
	}

	return session
}

func (a *ClaudeAdapter) GetSessionFiles(session model.Session) (map[string]string, error) {
	files := make(map[string]string)
	files["transcript.jsonl"] = session.Transcript

	claudeHome := GetClaudeHome()
	projectDir := filepath.Join(claudeHome, "projects", session.ProjectKey)

	sessionFolder := filepath.Join(projectDir, session.ID)
	if stat, err := os.Stat(sessionFolder); err == nil && stat.IsDir() {
		filepath.Walk(sessionFolder, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				rel, _ := filepath.Rel(sessionFolder, p)
				files[filepath.Join("tool-results", rel)] = p
			}
			return nil
		})
	}

	memoryFolder := filepath.Join(projectDir, "memory")
	if stat, err := os.Stat(memoryFolder); err == nil && stat.IsDir() {
		filepath.Walk(memoryFolder, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				rel, _ := filepath.Rel(memoryFolder, p)
				files[filepath.Join("memory", rel)] = p
			}
			return nil
		})
	}

	historyFolder := filepath.Join(claudeHome, "file-history", session.ID)
	if stat, err := os.Stat(historyFolder); err == nil && stat.IsDir() {
		filepath.Walk(historyFolder, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() {
				rel, _ := filepath.Rel(historyFolder, p)
				files[filepath.Join("file-history", rel)] = p
			}
			return nil
		})
	}

	return files, nil
}

func (a *ClaudeAdapter) InstallSession(manifest model.BundleManifest, files map[string][]byte, targetProjectPath string) error {
	claudeHome := GetClaudeHome()
	targetProjectKey := ComputeProjectKey(targetProjectPath, runtime.GOOS)

	targetProjDir := filepath.Join(claudeHome, "projects", targetProjectKey)
	if err := os.MkdirAll(targetProjDir, 0o755); err != nil {
		return err
	}

	for relPath, content := range files {
		var destPath string
		if relPath == "transcript.jsonl" {
			destPath = filepath.Join(targetProjDir, manifest.SessionID+".jsonl")
		} else if strings.HasPrefix(relPath, "tool-results") {
			sub := strings.TrimPrefix(relPath, "tool-results")
			sub = strings.TrimPrefix(sub, string(filepath.Separator))
			destPath = filepath.Join(targetProjDir, manifest.SessionID, sub)
		} else if strings.HasPrefix(relPath, "memory") {
			sub := strings.TrimPrefix(relPath, "memory")
			sub = strings.TrimPrefix(sub, string(filepath.Separator))
			destPath = filepath.Join(targetProjDir, "memory", sub)
		} else if strings.HasPrefix(relPath, "file-history") {
			sub := strings.TrimPrefix(relPath, "file-history")
			sub = strings.TrimPrefix(sub, string(filepath.Separator))
			destPath = filepath.Join(claudeHome, "file-history", manifest.SessionID, sub)
		} else {
			continue
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
