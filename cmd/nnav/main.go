package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	rootPath, err := notesRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "nnav: cannot determine notes dir:", err)
		os.Exit(1)
	}
	root, err := buildTree(rootPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nnav:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(newModel(root), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "nnav:", err)
		os.Exit(1)
	}
}

