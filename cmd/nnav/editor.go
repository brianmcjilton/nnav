package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Allowlist of supported editors. Restricting to this set avoids unexpected behavior
// from arbitrary binaries or command injections in config.
var allowedEditors = map[string]bool{
	"vim":   true,
	"vi":    true,
	"nano":  true,
	"nvim":  true,
	"hx":    true,
	"emacs": true,
}

// resolveEditor chooses which editor to launch when opening notes.
//
// Logic:
//   1. Read `editor` from ~/.nnav config (default to "vim" if unset).
//   2. Validate it is a *bare command name* (no slashes, spaces, or paths).
//      - Prevents users from setting dangerous values like "vim; rm -rf /" or "/usr/bin/vim".
//   3. Check it is in the `allowedEditors` list to ensure predictable UX.
//   4. Verify it exists in $PATH (via exec.LookPath).
//
// Returns: full binary path, no arguments (currently unused slice), or error.
func resolveEditor() (string, []string, error) {
	cfg, _ := loadConfig()
	raw := strings.TrimSpace(cfg["editor"]) // may be empty
	if raw == "" {
		raw = "vim" // default editor fallback
	}

	// Reject anything that looks like a path or contains whitespace/control chars.
	// This enforces a safe, consistent editor name policy.
	if raw != filepath.Base(raw) || strings.ContainsAny(raw, " /\t\\") {
		return "", nil, fmt.Errorf("invalid editor: %q (use a bare command name)", raw)
	}

	// Only allow explicitly permitted editors.
	if !allowedEditors[raw] {
		return "", nil, fmt.Errorf("editor not allowed: %q (allowed: vim, nvim, vi, nano, hx, emacs)", raw)
	}

	// Ensure the chosen editor is available in PATH before proceeding.
	path, err := exec.LookPath(raw)
	if err != nil {
		return "", nil, fmt.Errorf("editor not found in PATH: %q", raw)
	}

	return path, nil, nil
}

