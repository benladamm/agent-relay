package adapter_test

import (
	"testing"

	"github.com/benladamm/agent-relay/internal/adapter"
)

func TestDetection(t *testing.T) {
	reg := adapter.NewDefaultRegistry()
	harnesses := reg.DetectAllHarnesses()

	if len(harnesses) == 0 {
		t.Fatal("expected at least one harness registered")
	}

	for _, h := range harnesses {
		t.Logf("Harness %s: detected=%v, projects=%d, sessions=%d", h.DisplayName, h.Detected, h.ProjectCount, h.SessionCount)
	}

	projects := reg.DetectAllProjects("all", "")
	t.Logf("Total projects detected across harnesses: %d", len(projects))
	for _, p := range projects {
		t.Logf("  -> [%s] %s (%d sessions)", p.HarnessName, p.DisplayName, p.SessionCount)
	}
}
