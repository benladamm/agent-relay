package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/benladamm/agent-relay/internal/adapter"
	"github.com/benladamm/agent-relay/internal/model"
	"github.com/benladamm/agent-relay/internal/rewriter"
	"github.com/benladamm/agent-relay/internal/vcs"
)

// Export creates a .tar.gz bundle with the session files and current workspace diff.
func Export(ad adapter.Adapter, session model.Session, projectPath string, outputPath string) (string, error) {
	if projectPath == "" {
		projectPath = "."
	}
	absProject, err := filepath.Abs(projectPath)
	if err != nil {
		absProject = projectPath
	}

	if outputPath == "" {
		timestamp := time.Now().Format("20060102-150405")
		shortID := session.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		outputPath = fmt.Sprintf("relay-%s-%s.tar.gz", shortID, timestamp)
	}

	data, err := ExportBytes(ad, session, absProject)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return "", err
	}

	return outputPath, nil
}

// ExportBytes serializes the session and workspace diff directly to a gzip tar buffer.
func ExportBytes(ad adapter.Adapter, session model.Session, projectPath string) ([]byte, error) {
	absProject, err := filepath.Abs(projectPath)
	if err != nil {
		absProject = projectPath
	}

	sessionFiles, err := ad.GetSessionFiles(session)
	if err != nil {
		return nil, fmt.Errorf("failed collecting session files: %w", err)
	}

	gitState, _ := vcs.CaptureWorkspaceState(absProject)

	manifest := model.BundleManifest{
		Version:    "1.0.0",
		CreatedAt:  time.Now(),
		AgentName:  ad.ID(),
		SessionID:  session.ID,
		SourceOS:   runtime.GOOS,
		SourcePath: absProject,
		Metadata:   session,
	}
	if gitState != nil {
		manifest.HasGitDiff = len(gitState.Diff) > 0
		for f := range gitState.UntrackedFiles {
			manifest.UntrackedFiles = append(manifest.UntrackedFiles, f)
		}
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	if err := writeTarEntry(tw, "manifest.json", manifestBytes); err != nil {
		return nil, err
	}

	for relPath, srcPath := range sessionFiles {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}
		tarName := filepath.ToSlash(filepath.Join("session", relPath))
		if err := writeTarEntry(tw, tarName, data); err != nil {
			return nil, err
		}
	}

	if gitState != nil {
		if len(gitState.Diff) > 0 {
			if err := writeTarEntry(tw, "workspace/git-diff.patch", gitState.Diff); err != nil {
				return nil, err
			}
		}
		for relPath, data := range gitState.UntrackedFiles {
			tarName := filepath.ToSlash(filepath.Join("workspace/untracked", relPath))
			if err := writeTarEntry(tw, tarName, data); err != nil {
				return nil, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// PeekManifest inspects the manifest without extracting the full bundle.
func PeekManifest(bundleData []byte) (*model.BundleManifest, error) {
	gr, err := gzip.NewReader(bytes.NewReader(bundleData))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Name == "manifest.json" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			var m model.BundleManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, err
			}
			return &m, nil
		}
	}
	return nil, fmt.Errorf("manifest.json non trouvé dans le bundle")
}

// Import unpacks a bundle file from disk.
func Import(ad adapter.Adapter, bundlePath string, targetProjectPath string) (*model.BundleManifest, error) {
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, err
	}
	return ImportBytes(ad, data, targetProjectPath)
}

// ImportBytes unpacks a bundle from memory bytes.
func ImportBytes(ad adapter.Adapter, bundleData []byte, targetProjectPath string) (*model.BundleManifest, error) {
	if targetProjectPath == "" {
		targetProjectPath = "."
	}
	absTarget, err := filepath.Abs(targetProjectPath)
	if err != nil {
		absTarget = targetProjectPath
	}

	gr, err := gzip.NewReader(bytes.NewReader(bundleData))
	if err != nil {
		return nil, fmt.Errorf("invalid bundle (not gzip): %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	var manifest model.BundleManifest
	sessionFiles := make(map[string][]byte)
	gitState := &vcs.GitState{
		UntrackedFiles: make(map[string][]byte),
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}

		name := header.Name
		if name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("invalid manifest: %w", err)
			}
		} else if strings.HasPrefix(name, "session/") {
			rel := strings.TrimPrefix(name, "session/")
			sessionFiles[rel] = data
		} else if name == "workspace/git-diff.patch" {
			gitState.Diff = data
		} else if strings.HasPrefix(name, "workspace/untracked/") {
			rel := strings.TrimPrefix(name, "workspace/untracked/")
			gitState.UntrackedFiles[rel] = data
		}
	}

	if manifest.SessionID == "" {
		return nil, fmt.Errorf("bundle is missing valid manifest.json")
	}

	// 1. Rewrite transcript paths
	if transcript, ok := sessionFiles["transcript.jsonl"]; ok {
		rewritten := rewriter.RewriteTranscript(transcript, rewriter.Options{
			SourcePath: manifest.SourcePath,
			TargetPath: absTarget,
			SourceOS:   manifest.SourceOS,
			TargetOS:   runtime.GOOS,
		})
		sessionFiles["transcript.jsonl"] = rewritten
	}

	// 2. Install session files via adapter
	if err := ad.InstallSession(manifest, sessionFiles, absTarget); err != nil {
		return nil, fmt.Errorf("failed installing session: %w", err)
	}

	// 3. Restore workspace git changes
	_ = vcs.ApplyWorkspaceState(absTarget, gitState)

	return &manifest, nil
}

func writeTarEntry(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0o644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
