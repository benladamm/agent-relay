package adapter

import (
	"fmt"
	"strings"

	"github.com/benladamm/agent-relay/internal/adapter/aider"
	"github.com/benladamm/agent-relay/internal/adapter/antigravity"
	"github.com/benladamm/agent-relay/internal/adapter/claudecode"
	"github.com/benladamm/agent-relay/internal/adapter/cline"
	"github.com/benladamm/agent-relay/internal/adapter/cursor"
	"github.com/benladamm/agent-relay/internal/adapter/windsurf"
	"github.com/benladamm/agent-relay/internal/model"
)

// Registry manages the set of supported AI coding harnesses.
type Registry struct {
	adapters map[string]Adapter
	order    []string
}

func NewDefaultRegistry() *Registry {
	r := &Registry{
		adapters: make(map[string]Adapter),
	}

	r.Register(claudecode.New())
	r.Register(antigravity.New())
	r.Register(cursor.New())
	r.Register(windsurf.New())
	r.Register(cline.New())
	r.Register(aider.New())

	return r
}

func (r *Registry) Register(a Adapter) {
	r.adapters[a.ID()] = a
	r.order = append(r.order, a.ID())
}

func (r *Registry) Get(id string) (Adapter, bool) {
	a, ok := r.adapters[id]
	return a, ok
}

func (r *Registry) DetectAllHarnesses() []model.HarnessInfo {
	var list []model.HarnessInfo
	for _, id := range r.order {
		ad := r.adapters[id]
		info, err := ad.DetectHarness()
		if err == nil {
			list = append(list, info)
		}
	}
	return list
}

func (r *Registry) DetectAllProjects(harnessFilter string, searchFilter string) []model.Project {
	var all []model.Project
	for _, id := range r.order {
		if harnessFilter != "" && harnessFilter != "all" && id != harnessFilter {
			continue
		}
		ad := r.adapters[id]
		projects, err := ad.DetectProjects()
		if err == nil {
			all = append(all, projects...)
		}
	}

	if searchFilter == "" {
		return all
	}

	searchLower := strings.ToLower(searchFilter)
	var filtered []model.Project
	for _, p := range all {
		matchName := strings.Contains(strings.ToLower(p.DisplayName), searchLower)
		matchPath := strings.Contains(strings.ToLower(p.Path), searchLower)
		matchHarness := strings.Contains(strings.ToLower(p.HarnessName), searchLower)
		if matchName || matchPath || matchHarness {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (r *Registry) GetSessions(project model.Project) ([]model.Session, error) {
	ad, ok := r.adapters[project.HarnessID]
	if !ok {
		return nil, fmt.Errorf("unknown harness: %s", project.HarnessID)
	}
	return ad.GetProjectSessions(project.ProjectKey)
}
