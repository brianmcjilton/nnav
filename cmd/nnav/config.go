package main

import (
	"os"
	"path/filepath"
	"strings"
)

var (
	// Default directory name under $HOME if not overridden by config.
	defaultNotesSubdir = "notes"

	// Restrict which file extensions are considered valid "note" files.
	allowedExts = map[string]bool{".md": true, ".txt": true}

	// User-specific config file (~/.nnav) that defines preferences like notesdir and editor.
	userConfigFile = ".nnav" // located in the user's home dir
)

// ensureConfig guarantees that ~/.nnav exists with secure permissions.
// If missing, it creates the file with default values; if present, it enforces
// restrictive permissions (0600). This prevents accidental world-readable configs.
func ensureConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(home, userConfigFile)

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// Config file does not exist → create with defaults.
		// Security note: permissions are explicitly set to 0600 so only the user can read/write.
		// #nosec G304 -- cfgPath is derived from $HOME and not attacker-controlled.
		f, err := os.OpenFile(cfgPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return "", err
		}
		defer f.Close()

		// Write minimal defaults plus inline documentation for the user.
		_, _ = f.WriteString(`# nnav configuration
# notesdir: path to your notes directory (e.g., ~/notes). Must be readable by your user.
# editor: which editor to launch. Allowed values: vim, nvim, vi, nano, hx, emacs
notesdir=~/notes
editor=vim
`)
	} else if err == nil {
		// Config file exists → ensure permissions are still locked down.
		_ = os.Chmod(cfgPath, 0o600)
	}
	return cfgPath, nil
}

// loadConfig parses ~/.nnav into a map of key=value settings.
// Lines starting with "#" or blank lines are ignored.
// Invalid lines are skipped silently rather than failing the load.
func loadConfig() (map[string]string, error) {
	cfgPath, err := ensureConfig()
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- cfgPath is computed internally, not controlled by user input.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	m := map[string]string{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // skip comments and empty lines
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue // malformed line → ignore
		}
		k := strings.ToLower(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		m[k] = v
	}
	return m, nil
}

// expandTilde resolves paths starting with "~" to the user’s home directory.
// This allows user configs to specify relative paths like "~/notes".
func expandTilde(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
	}
	return p, nil
}

// notesRoot determines the effective notes directory.
// Priority: (1) "notesdir" in ~/.nnav config, if valid, else (2) fallback to ~/notes.
func notesRoot() (string, error) {
	cfg, err := loadConfig()
	if err == nil {
		if v := strings.TrimSpace(cfg["notesdir"]); v != "" {
			return expandTilde(v)
		}
	}

	// Fallback: if no config or no notesdir set, use ~/notes.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultNotesSubdir), nil
}

