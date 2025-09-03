package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var searchTerm string
	// Optional search term filters the tree to notes containing the keyword.
	if len(os.Args) > 1 {
		searchTerm = os.Args[1]
	}

	// Determine the root directory where notes are stored.
	// If this cannot be resolved, the program cannot function, so exit immediately.
	rootPath, err := notesRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nnav: cannot determine notes dir:", err)
		os.Exit(1)
	}

	// Build an in-memory tree representation of the notes directory.
	// This structure drives the TUI navigation model.
	root, err := buildTree(rootPath, searchTerm)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nnav:", err)
		os.Exit(1)
	}

	// Initialize the Bubble Tea program with the model created from the notes tree.
	// tea.WithAltScreen() ensures the TUI runs in a fullscreen alternate buffer
	// (so it doesn't clutter the user's normal terminal scrollback).
	p := tea.NewProgram(newModel(root, searchTerm), tea.WithAltScreen())

	// Start the program’s event loop.
	// If the loop exits with an error, report it to stderr and terminate.
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "nnav:", err)
		os.Exit(1)
	}
}

