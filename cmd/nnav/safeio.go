package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// safeJoinWithin ensures that a user-supplied path resolves *within* a trusted base directory.
// Protections:
//   - Disallows absolute paths (must be relative to base).
//   - Cleans inputs to remove redundant ../ or ./ elements.
//   - Validates final resolved path does not traverse outside base (via filepath.Rel).
//   - Evaluates symlinks to defend against symlink-based directory escapes.
//   - Returns the absolute, sanitized path if safe; error otherwise.
func safeJoinWithin(base, userPath string) (string, error) {
	clean := filepath.Clean(userPath)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	joined := filepath.Join(base, clean)

	// Reject if the resulting path points outside of base.
	rel, err := filepath.Rel(base, joined)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." && clean == ".." {
		return "", fmt.Errorf("path escapes base dir")
	}

	// Resolve symlinks in base and joined path to prevent escape via crafted symlinks.
	absBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.EvalSymlinks(joined)
	if err != nil {
		// If target doesn’t exist yet, fall back to the non-resolved absolute path.
		absJoined = joined
	}

	// Add trailing separator to avoid false prefix matches (e.g., /tmp/foo vs /tmp/foobar).
	baseWithSep := absBase + string(os.PathSeparator)
	joinedWithSep := absJoined + string(os.PathSeparator)

	if !strings.HasPrefix(joinedWithSep, baseWithSep) && absJoined != absBase {
		return "", fmt.Errorf("symlink escape detected")
	}
	return absJoined, nil
}

// safePathWithinNotes ensures a path is located under the notes root directory.
// Uses safeJoinWithin() to sanitize relative paths. Returns (path, true) if safe,
// else ("", false). This provides a centralized gatekeeper for file system access.
func safePathWithinNotes(p string) (string, bool) {
	root, err := notesRoot()
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", false
	}
	safe, err := safeJoinWithin(root, rel)
	if err != nil {
		return "", false
	}
	return safe, true
}

// isReadableFile checks if a given path under notesRoot is an accessible file.
// Returns true if the file exists and at least 1 byte can be read.
// Uses safePathWithinNotes() to prevent traversal outside notesRoot.
func isReadableFile(path string) bool {
	if safe, ok := safePathWithinNotes(path); ok {
		f, err := os.Open(safe)
		if err != nil {
			return false
		}
		defer f.Close()

		// Attempt to read 1 byte to confirm readability.
		buf := make([]byte, 1)
		_, _ = f.Read(buf)
		return true
	}
	return false
}

// isListableDir checks if a given path under notesRoot is a directory that can be opened and listed.
// Returns true if the directory exists and at least one entry can be read (or EOF if empty).
// Prevents directory traversal via safePathWithinNotes().
func isListableDir(path string) bool {
	if safe, ok := safePathWithinNotes(path); ok {
		d, err := os.Open(safe)
		if err != nil {
			return false
		}
		defer d.Close()

		// Try reading a single entry to confirm it can be listed.
		_, err = d.Readdirnames(1)
		if err != nil && !errors.Is(err, io.EOF) {
			return false
		}
		return true
	}
	return false
}

