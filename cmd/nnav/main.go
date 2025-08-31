package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Config ---
var (
	defaultNotesSubdir = "notes"
	allowedExts        = map[string]bool{".md": true, ".txt": true}

	userConfigFile = ".nnav" // located in the user's home dir
)

// ensureConfig makes sure ~/.nnav exists, creating it with default notesdir if missing.
func ensureConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(home, userConfigFile)

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// Create file with default content
		f, err := os.Create(cfgPath)
		if err != nil {
			return "", err
		}
		defer f.Close()
		_, _ = f.WriteString("notesdir=~/notes\n")
	}
	return cfgPath, nil
}

// loadNotesDirFromConfig reads notesdir from ~/.nnav
func loadNotesDirFromConfig() (string, error) {
	cfgPath, err := ensureConfig()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "notesdir=") {
			v := strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
			return expandTilde(v)
		}
	}
	return "", nil
}

// expandTilde expands a leading ~ or ~/… to the user's home directory.
// (Intentionally does NOT expand arbitrary environment variables.)
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

// notesRoot resolves the notes directory using:
// 1) notesdir from ~/.nnav (creating the file on first run)
// 2) fallback: ~/notes
func notesRoot() (string, error) {
	if v, err := loadNotesDirFromConfig(); err == nil && v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultNotesSubdir), nil
}

var headingRE = regexp.MustCompile(`^\s*#{1,6}\s*(.+?)\s*$`)

func scanTitle(p string) string {
	f, err := os.Open(p)
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
		if e.IsDir() {
			n := &Node{Name: name, Path: p, IsDir: true}
			nodes = append(nodes, n)
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !allowedExts[ext] {
			continue
		}
		title := scanTitle(p)
		n := &Node{Name: name, Path: p, Title: title}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// resolveEditor returns a command slice to launch the preferred editor.
// Priority: $VISUAL -> $EDITOR -> first available of [vim, vi, nano].
// (This keeps editor selection flexible; if you want zero-env usage, hard-code vim.)
func resolveEditor() []string {
	candidates := []string{}
	if v := strings.TrimSpace(os.Getenv("VISUAL")); v != "" {
		candidates = append(candidates, v)
	}
	if v := strings.TrimSpace(os.Getenv("EDITOR")); v != "" {
		candidates = append(candidates, v)
	}
	candidates = append(candidates, "vim", "vi", "nano")

	for _, c := range candidates {
		parts := strings.Fields(c)
		if len(parts) == 0 {
			continue
		}
		if _, err := exec.LookPath(parts[0]); err == nil {
			return parts
		}
	}
	return []string{"/bin/vi"}
}

// --- TUI types/model ---

type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Expanded bool
	Title    string
	Children []*Node
}

type Visible struct {
	N     *Node
	Depth int
}

type model struct {
	root    *Node
	cursor  int
	visible []Visible
	status  string
	width   int
	height  int
}

func newModel(root *Node) model {
	help := "↑/↓ or j/k move • →/l expand • ←/h collapse • enter open • r reload • q quit"
	m := model{root: root, cursor: 0, status: help}
	m.recompute()
	return m
}

func (m *model) recompute() {
	m.visible = m.visible[:0]
	flatten(m.root, 0, &m.visible)
	if m.cursor >= len(m.visible) {
		m.cursor = max(0, len(m.visible)-1)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "right", "l":
			cur := m.visible[m.cursor].N
			if cur.IsDir && !cur.Expanded {
				if err := expandIfNeeded(cur); err != nil {
					m.status = "error: " + err.Error()
				} else {
					m.recompute()
				}
			}
		case "left", "h":
			cur := m.visible[m.cursor].N
			if cur.IsDir && cur.Expanded {
				cur.Expanded = false
				m.recompute()
			}
		case "enter":
			cur := m.visible[m.cursor].N
			if !cur.IsDir {
				ed := resolveEditor()
				cmd := exec.Command(ed[0], append(ed[1:], cur.Path)...)
				cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
				// Exit alt screen for external editor, then re-enter.
				fmt.Print("\x1b[?1049l")
				_ = cmd.Run()
				fmt.Print("\x1b[?1049h")
				// Rebuild after editor closes
				rootPath, _ := notesRoot()
				if root, err := buildTree(rootPath); err == nil {
					m.root = root
					m.cursor = 0
					m.recompute()
					m.status = "opened with: " + strings.Join(ed, " ")
				} else {
					m.status = "reload failed: " + err.Error()
				}
			}
		case "r":
			rootPath, _ := notesRoot()
			if root, err := buildTree(rootPath); err == nil {
				m.root = root
				m.cursor = 0
				m.recompute()
				m.status = "reloaded at " + time.Now().Format("15:04:05")
			} else {
				m.status = "reload failed: " + err.Error()
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorStyle := lipgloss.NewStyle().Reverse(true)

	b.WriteString(titleStyle.Render("nnav - Notes Navigator"))
	b.WriteString("\n\n")

	for i, v := range m.visible {
		line := renderLine(v)
		if i == m.cursor {
			line = cursorStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(muted.Render(m.status))
	b.WriteString("\n")
	return b.String()
}

func renderLine(v Visible) string {
	indent := strings.Repeat("  ", v.Depth)
	prefix := "  "
	name := v.N.Name
	if v.N.IsDir {
		if v.N.Expanded {
			prefix = "▾ "
		} else {
			prefix = "▸ "
		}
	} else {
		prefix = "• "
	}
	title := v.N.Title
	if title != "" {
		name = fmt.Sprintf("%s — %s", name, title)
	}
	return indent + prefix + name
}

func flatten(n *Node, depth int, out *[]Visible) {
	*out = append(*out, Visible{N: n, Depth: depth})
	if n.IsDir && n.Expanded {
		for _, c := range n.Children {
			flatten(c, depth+1, out)
		}
	}
}

func expandIfNeeded(n *Node) error {
	if !n.IsDir {
		return nil
	}
	if n.Expanded && len(n.Children) > 0 {
		return nil
	}
	if len(n.Children) == 0 {
		kids, err := readDirNodes(n.Path)
		if err != nil {
			return err
		}
		n.Children = kids
	}
	n.Expanded = true
	return nil
}

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

