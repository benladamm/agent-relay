package vcs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitState contains uncommitted diff and untracked file contents.
type GitState struct {
	Diff           []byte            `json:"diff"`
	UntrackedFiles map[string][]byte `json:"untracked_files"`
}

// CaptureWorkspaceState extracts uncommitted git changes and untracked files.
func CaptureWorkspaceState(repoDir string) (*GitState, error) {
	if repoDir == "" {
		repoDir = "."
	}

	state := &GitState{
		UntrackedFiles: make(map[string][]byte),
	}

	// 1. Get git diff HEAD
	cmdDiff := exec.Command("git", "diff", "HEAD")
	cmdDiff.Dir = repoDir
	var outDiff bytes.Buffer
	cmdDiff.Stdout = &outDiff
	if err := cmdDiff.Run(); err == nil {
		state.Diff = outDiff.Bytes()
	}

	// 2. Find untracked files
	cmdStatus := exec.Command("git", "status", "--porcelain")
	cmdStatus.Dir = repoDir
	var outStatus bytes.Buffer
	cmdStatus.Stdout = &outStatus
	if err := cmdStatus.Run(); err == nil {
		lines := strings.Split(outStatus.String(), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "??") {
				relPath := strings.TrimSpace(strings.TrimPrefix(line, "??"))
				fullPath := filepath.Join(repoDir, relPath)
				data, err := os.ReadFile(fullPath)
				if err == nil {
					state.UntrackedFiles[relPath] = data
				}
			}
		}
	}

	return state, nil
}

// ApplyWorkspaceState writes untracked files and applies the git diff patch.
func ApplyWorkspaceState(repoDir string, state *GitState) error {
	if state == nil {
		return nil
	}

	// 1. Write untracked files
	for relPath, data := range state.UntrackedFiles {
		dest := filepath.Join(repoDir, relPath)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("failed creating dir for %s: %w", relPath, err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("failed writing untracked file %s: %w", relPath, err)
		}
	}

	// 2. Apply git diff if present
	if len(state.Diff) > 0 {
		cmdApply := exec.Command("git", "apply", "--ignore-whitespace", "--whitespace=nowarn", "-")
		cmdApply.Dir = repoDir
		cmdApply.Stdin = bytes.NewReader(state.Diff)
		var errBuf bytes.Buffer
		cmdApply.Stderr = &errBuf
		if err := cmdApply.Run(); err != nil {
			// Don't fail hard on minor patch rejection, save as .patch file
			patchFile := filepath.Join(repoDir, "agent-relay-unapplied.patch")
			_ = os.WriteFile(patchFile, state.Diff, 0o644)
			return fmt.Errorf("git apply warning: %s (patch saved to %s)", strings.TrimSpace(errBuf.String()), patchFile)
		}
	}

	return nil
}
