package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var allowedEditors = map[string]bool{
	"vim":   true,
	"vi":    true,
	"nano":  true,
	"nvim":  true,
	"hx":    true,
	"emacs": true,
}

// resolveEditor returns a validated editor binary path from ~/.nnav (default vim).
// Disallows spaces/paths; only bare command names from allowlist are accepted.
func resolveEditor() (string, []string, error) {
	cfg, _ := loadConfig()
	raw := strings.TrimSpace(cfg["editor"]) // may be empty
	if raw == "" {
		raw = "vim"
	}
	// Reject spaces, slashes, or tabs; only allow bare command names
	if raw != filepath.Base(raw) || strings.ContainsAny(raw, " /\t\\") {
		return "", nil, fmt.Errorf("invalid editor: %q (use a bare command name)", raw)
	}
	if !allowedEditors[raw] {
		return "", nil, fmt.Errorf("editor not allowed: %q (allowed: vim, nvim, vi, nano, hx, emacs)", raw)
	}
	path, err := exec.LookPath(raw)
	if err != nil {
		return "", nil, fmt.Errorf("editor not found in PATH: %q", raw)
	}
	return path, nil, nil
}

