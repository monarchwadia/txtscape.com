package pages

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	fileNameRe   = regexp.MustCompile(`^[a-z0-9_-]{1,245}\.txt$`)
	folderPartRe = regexp.MustCompile(`^[a-z0-9_-]{1,10}$`)
)

const maxDepth = 10

// ParsedPath represents a validated path split into folder and filename.
type ParsedPath struct {
	FolderPath string // e.g. "/" or "/blog/" or "/blog/2026/"
	FileName   string // e.g. "hello.txt"
}

// ParsePath validates and splits a raw URL path like "hello.txt" or "blog/2026/post.txt"
// into a folder path and filename. The raw path should NOT have a leading slash.
func ParsePath(raw string) (*ParsedPath, error) {
	if raw == "" {
		return nil, fmt.Errorf("path is empty")
	}

	// Must not contain backslashes
	if strings.Contains(raw, `\`) {
		return nil, fmt.Errorf("path must not contain backslashes")
	}

	// Must not contain path traversal
	if strings.Contains(raw, "..") {
		return nil, fmt.Errorf("path must not contain '..'")
	}

	parts := strings.Split(raw, "/")
	fileName := parts[len(parts)-1]
	folderParts := parts[:len(parts)-1]

	if !fileNameRe.MatchString(fileName) {
		return nil, fmt.Errorf("invalid filename %q: must be lowercase alphanumeric/hyphens/underscores ending in .txt", fileName)
	}

	if len(folderParts) >= maxDepth {
		return nil, fmt.Errorf("path exceeds maximum depth of %d levels", maxDepth)
	}

	for _, p := range folderParts {
		if !folderPartRe.MatchString(p) {
			return nil, fmt.Errorf("invalid folder name %q: must be 1-10 lowercase alphanumeric/hyphens/underscores", p)
		}
	}

	folderPath := "/"
	if len(folderParts) > 0 {
		folderPath = "/" + strings.Join(folderParts, "/") + "/"
	}

	return &ParsedPath{FolderPath: folderPath, FileName: fileName}, nil
}
