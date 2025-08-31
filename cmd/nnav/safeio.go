package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Ensure a path remains within base, and guard against symlink escapes.
func safeJoinWithin(base, userPath string) (string, error) {
	clean := filepath.Clean(userPath)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	joined := filepath.Join(base, clean)
	rel, err := filepath.Rel(base, joined)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." && clean == ".." {
		return "", fmt.Errorf("path escapes base dir")
	}
	absBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", err
	}
	absJoined, err := filepath.EvalSymlinks(joined)
	if err != nil {
		// If file doesn't exist yet, fall back to absolute
		absJoined = joined
	}
	baseWithSep := absBase + string(os.PathSeparator)
	joinedWithSep := absJoined + string(os.PathSeparator)
	if !strings.HasPrefix(joinedWithSep, baseWithSep) && absJoined != absBase {
		return "", fmt.Errorf("symlink escape detected")
	}
	return absJoined, nil
}

// safePathWithinNotes returns a sanitized absolute path under notesRoot.
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

func isReadableFile(path string) bool {
	if safe, ok := safePathWithinNotes(path); ok {
		f, err := os.Open(safe)
		if err != nil {
			return false
		}
		defer f.Close()
		buf := make([]byte, 1)
		_, _ = f.Read(buf)
		return true
	}
	return false
}

func isListableDir(path string) bool {
	if safe, ok := safePathWithinNotes(path); ok {
		d, err := os.Open(safe)
		if err != nil {
			return false
		}
		defer d.Close()
		_, err = d.Readdirnames(1)
		if err != nil && !errors.Is(err, io.EOF) {
			return false
		}
		return true
	}
	return false
}

