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

var headingRE = regexp.MustCompile(`^\s*#{1,6}\s*(.+?)\s*$`)

type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Expanded bool
	Title    string
	Children []*Node
}

func scanTitle(p string) string {
	// Open only after validating path remains under notesRoot
	if safe, ok := safePathWithinNotes(p); ok {
		f, err := os.Open(safe)
		if err != nil {
			return ""
		}
		defer f.Close()

		s := bufio.NewScanner(f)
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
	rootNode := &Node{Name: filepath.Base(root), Path: root, IsDir: true, Expanded: true}
	children, err := readDirNodes(root)
	if err != nil {
		return nil, err
	}
	rootNode.Children = children
	return rootNode, nil
}

func readDirNodes(dir string) ([]*Node, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
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
			continue // skip entries we can't stat
		}
		if info.IsDir() {
			if !isListableDir(p) {
				continue // don't display dirs we can't read
			}
			n := &Node{Name: name, Path: p, IsDir: true}
			nodes = append(nodes, n)
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !allowedExts[ext] {
			continue
		}
		if !isReadableFile(p) {
			continue // skip unreadable files
		}
		title := scanTitle(p)
		n := &Node{Name: name, Path: p, Title: title}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

