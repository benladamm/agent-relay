package aider

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benladamm/agent-relay/internal/model"
)

// AiderAdapter implements adapter.Adapter for Aider.
type AiderAdapter struct{}

func New() *AiderAdapter {
	return &AiderAdapter{}
}

func (a *AiderAdapter) ID() string {
	return "aider"
}

func (a *AiderAdapter) DisplayName() string {
	return "Aider"
}

func (a *AiderAdapter) DetectHarness() (model.HarnessInfo, error) {
	home, _ := os.UserHomeDir()
	info := model.HarnessInfo{
		ID:          a.ID(),
		DisplayName: a.DisplayName(),
		HomePath:    home,
		Detected:    false,
	}

	// Check if ~/.aider or git repositories with .aider.chat.history.md exist
	aiderHome := filepath.Join(home, ".aider")
	if stat, err := os.Stat(aiderHome); err == nil && stat.IsDir() {
		info.Detected = true
	}

	projects, _ := a.DetectProjects()
	if len(projects) > 0 {
		info.Detected = true
		info.ProjectCount = len(projects)
		for _, p := range projects {
			info.SessionCount += p.SessionCount
		}
	}

	return info, nil
}

func (a *AiderAdapter) DetectProjects() ([]model.Project, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil
	}

	searchRoots := []string{
		filepath.Join(home, "Documents", "GitHub"),
		filepath.Join(home, "GitHub"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "dev"),
	}

	var projects []model.Project
	for _, root := range searchRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			projPath := filepath.Join(root, e.Name())
			historyFile := filepath.Join(projPath, ".aider.chat.history.md")
			if fi, err := os.Stat(historyFile); err == nil {
				projects = append(projects, model.Project{
					HarnessID:    a.ID(),
					HarnessName:  a.DisplayName(),
					ProjectKey:   e.Name(),
					Path:         projPath,
					DisplayName:  e.Name(),
					LastModified: fi.ModTime(),
					SessionCount: 1,
				})
			}
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

func (a *AiderAdapter) GetProjectSessions(projectKey string) ([]model.Session, error) {
	projects, _ := a.DetectProjects()
	for _, p := range projects {
		if p.ProjectKey == projectKey {
			histFile := filepath.Join(p.Path, ".aider.chat.history.md")
			fi, err := os.Stat(histFile)
			if err != nil {
				return nil, err
			}

			summary := extractAiderSummary(histFile)

			session := model.Session{
				ID:           projectKey,
				AgentName:    a.ID(),
				ProjectPath:  p.Path,
				ProjectKey:   projectKey,
				Transcript:   histFile,
				Status:       model.StatusCompleted,
				LastModified: fi.ModTime(),
				TotalBytes:   fi.Size(),
				Summary:      summary,
			}
			return []model.Session{session}, nil
		}
	}
	return nil, nil
}

func extractAiderSummary(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "Aider Conversation History"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lastPrompt string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#### ") {
			lastPrompt = strings.TrimPrefix(line, "#### ")
		}
	}
	if lastPrompt != "" {
		if len(lastPrompt) > 85 {
			lastPrompt = lastPrompt[:82] + "..."
		}
		return lastPrompt
	}
	return "Aider Chat Session"
}

func (a *AiderAdapter) GetSessionFiles(session model.Session) (map[string]string, error) {
	files := make(map[string]string)
	files[".aider.chat.history.md"] = session.Transcript

	inputHist := filepath.Join(session.ProjectPath, ".aider.input.history")
	if _, err := os.Stat(inputHist); err == nil {
		files[".aider.input.history"] = inputHist
	}

	return files, nil
}

func (a *AiderAdapter) InstallSession(manifest model.BundleManifest, files map[string][]byte, targetProjectPath string) error {
	for relPath, content := range files {
		dest := filepath.Join(targetProjectPath, relPath)
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
