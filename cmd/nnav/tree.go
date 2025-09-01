package main

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// headingRE matches Markdown headings (# through ######) and captures the text.
// Used to extract the first meaningful title from note files.
var headingRE = regexp.MustCompile(`^\s*#{1,6}\s*(.+?)\s*$`)

// Node represents a file or directory in the notes tree.
//   - Name: display name (filename or dir).
//   - Path: full filesystem path.
//   - IsDir: whether this is a directory.
//   - Expanded: whether the directory is expanded in the TUI.
//   - Title: optional, extracted title from the file’s first Markdown heading.
//   - Children: nested files/directories if IsDir is true.
type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Expanded bool
	Title    string
	Children []*Node
}

// scanTitle extracts the first Markdown heading from a note file.
// Uses safePathWithinNotes to ensure the file is inside notesRoot.
// Returns the heading text (without #) or "" if none found.
//
// Implementation details:
//   - Uses bufio.Scanner with increased buffer size to handle long lines.
//   - Stops at the first heading encountered.
func scanTitle(p string) string {
	// Validate path safety before opening.
	if safe, ok := safePathWithinNotes(p); ok {
		f, err := os.Open(safe)
		if err != nil {
			return ""
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		// Allow lines up to 10MB to avoid truncation of unusually long headings.
		s.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

		for s.Scan() {
			line := s.Text()
			if m := headingRE.FindStringSubmatch(line); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

// buildTree constructs a Node tree starting at the given root path.
// Validates that root exists, is a directory, and is listable by the user.
// Returns a Node with populated children for the top level.
func buildTree(root string) (*Node, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("notesdir must point to a directory")
	}
	if !isListableDir(root) {
		return nil, errors.New("cannot read notesdir: " + root)
	}

	// Root node is always marked Expanded so children are shown initially.
	rootNode := &Node{Name: filepath.Base(root), Path: root, IsDir: true, Expanded: true}

	children, err := readDirNodes(root)
	if err != nil {
		return nil, err
	}
	rootNode.Children = children
	return rootNode, nil
}

// readDirNodes scans the contents of a directory and returns child Nodes.
//
// Ordering:
//   - Directories appear before files.
//   - Entries are case-insensitively sorted by name.
//
// Filtering:
//   - Skips entries that cannot be stat()’d.
//   - Skips dirs that cannot be listed (permissions).
//   - Skips files without allowed extensions (.md, .txt).
//   - Skips unreadable files.
//   - Extracts a title for note files via scanTitle().
func readDirNodes(dir string) ([]*Node, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Sort: directories first, then alphabetical (case-insensitive).
	sort.Slice(ents, func(i, j int) bool {
		a, b := ents[i], ents[j]
		if a.IsDir() && !b.IsDir() {
			return true
		}
		if !a.IsDir() && b.IsDir() {
			return false
		}
		return strings.ToLower(a.Name()) < strings.ToLower(b.Name())
	})

	nodes := make([]*Node, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		p := filepath.Join(dir, name)

		info, err := e.Info()
		if err != nil {
			continue // skip entries we can’t stat
		}

		if info.IsDir() {
			if !isListableDir(p) {
				continue // skip unreadable directories
			}
			n := &Node{Name: name, Path: p, IsDir: true}
			nodes = append(nodes, n)
			continue
		}

		// File: only accept known note extensions.
		ext := strings.ToLower(filepath.Ext(name))
		if !allowedExts[ext] {
			continue
		}
		if !isReadableFile(p) {
			continue // skip unreadable files
		}

		// Extract optional title from file content.
		title := scanTitle(p)
		n := &Node{Name: name, Path: p, Title: title}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

