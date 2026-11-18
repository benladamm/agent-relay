package rewriter

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
)

// Options holds parameters for rewriting paths in session transcripts.
type Options struct {
	SourcePath string
	TargetPath string
	SourceOS   string
	TargetOS   string
}

// RewriteTranscript modifies session data to update file and project paths for the target OS.
func RewriteTranscript(content []byte, opts Options) []byte {
	if opts.SourcePath == "" || opts.TargetPath == "" || opts.SourcePath == opts.TargetPath {
		return content
	}

	src := opts.SourcePath
	target := opts.TargetPath

	// 1. Windows double-escaped path: C:\\Users\\... -> /home/... or D:\\...
	winEscapedSrc := strings.ReplaceAll(src, `\`, `\\`)
	var winEscapedTarget string
	if opts.TargetOS == "windows" {
		winEscapedTarget = strings.ReplaceAll(target, `\`, `\\`)
	} else {
		// On POSIX target, use forward slashes
		winEscapedTarget = filepath.ToSlash(target)
	}

	content = bytes.ReplaceAll(content, []byte(winEscapedSrc), []byte(winEscapedTarget))
	content = bytes.ReplaceAll(content, []byte(strings.ToLower(winEscapedSrc)), []byte(winEscapedTarget))

	// 2. Git-Bash / Cygwin style: /c/Users/... -> /home/...
	if len(src) >= 2 && src[1] == ':' {
		drive := strings.ToLower(string(src[0]))
		rest := filepath.ToSlash(src[2:])
		gitBashSrc := "/" + drive + rest
		content = bytes.ReplaceAll(content, []byte(gitBashSrc), []byte(filepath.ToSlash(target)))
	}

	// 3. Normal backslash path: C:\Users\...
	content = bytes.ReplaceAll(content, []byte(src), []byte(target))
	content = bytes.ReplaceAll(content, []byte(strings.ToLower(src)), []byte(target))

	// 4. Forward slash path: C:/Users/...
	forwardSrc := filepath.ToSlash(src)
	forwardTarget := filepath.ToSlash(target)
	content = bytes.ReplaceAll(content, []byte(forwardSrc), []byte(forwardTarget))
	content = bytes.ReplaceAll(content, []byte(strings.ToLower(forwardSrc)), []byte(forwardTarget))

	// 5. Clean up any leftover synthetic rate limit stop sequences if user requests
	// We preserve the limit notification so the model knows it was resumed cleanly
	return content
}

// SanitizeForProjectKey formats any folder path to match Claude Code's project key convention.
func SanitizeForProjectKey(p string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]`)
	clean := filepath.Clean(p)
	return reg.ReplaceAllString(clean, "-")
}
