package main

import (
	"os"
	"path/filepath"
	"strings"
)

var (
	defaultNotesSubdir = "notes"
	allowedExts        = map[string]bool{".md": true, ".txt": true}

	userConfigFile = ".nnav" // located in the user's home dir
)

// ensureConfig makes sure ~/.nnav exists with secure perms and defaults.
func ensureConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(home, userConfigFile)

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// Create file with default content, restrictive perms
		// #nosec G304 -- cfgPath is derived from $HOME/.nnav and not attacker-controlled
		f, err := os.OpenFile(cfgPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return "", err
		}
		defer f.Close()
		_, _ = f.WriteString(`# nnav configuration
# notesdir: path to your notes directory (e.g., ~/notes). Must be readable by your user.
# editor: which editor to launch. Allowed values: vim, nvim, vi, nano, hx, emacs
notesdir=~/notes
editor=vim
`)
	} else if err == nil {
		// Tighten perms if too loose
		_ = os.Chmod(cfgPath, 0o600)
	}
	return cfgPath, nil
}

// loadConfig reads simple key=value pairs from ~/.nnav
func loadConfig() (map[string]string, error) {
	cfgPath, err := ensureConfig()
	if err != nil {
		return nil, err
	}
	// #nosec G304 -- cfgPath is computed as $HOME/.nnav and not user-controlled
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		m[k] = v
	}
	return m, nil
}

// expandTilde expands a leading ~ or ~/… to the user's home directory.
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

// notesRoot resolves the notes directory using ~/.nnav or fallback: ~/notes
func notesRoot() (string, error) {
	cfg, err := loadConfig()
	if err == nil {
		if v := strings.TrimSpace(cfg["notesdir"]); v != "" {
			return expandTilde(v)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultNotesSubdir), nil
}

