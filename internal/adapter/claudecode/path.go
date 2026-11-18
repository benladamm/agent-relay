package claudecode

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var nonAlphaNumRegex = regexp.MustCompile(`[^a-zA-Z0-9]`)

// ComputeProjectKey calculates the project key Claude Code uses in ~/.claude/projects/
func ComputeProjectKey(absPath string, targetOS string) string {
	clean := filepath.Clean(absPath)
	if targetOS == "" {
		targetOS = runtime.GOOS
	}

	if targetOS == "windows" {
		// e.g. C:\Users\... -> c--Users-...
		if len(clean) >= 2 && clean[1] == ':' {
			drive := strings.ToLower(string(clean[0]))
			rest := clean[2:]
			clean = drive + "-" + rest
		}
	}

	sanitized := nonAlphaNumRegex.ReplaceAllString(clean, "-")
	return sanitized
}
