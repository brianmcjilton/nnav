package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const helpText = "↑/↓ move • → expand • ← collapse • <enter> open • <q> quit"

const minTopMargin = 2   // lines to keep above cursor
const minBottomMargin = 2 // lines to keep below cursor

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
	scroll  int // top index of visible window
}

// message sent after we return from the editor
type resumedMsg struct{}

func newModel(root *Node) model {
	m := model{root: root, cursor: 0, status: helpText}
	m.recompute()
	return m
}

// displayName returns what we render for a node:
// - files: Title if present, else filename
// - dirs: directory name
func displayName(n *Node) string {
	if n.IsDir {
		return n.Name
	}
	if t := strings.TrimSpace(n.Title); t != "" {
		return t
	}
	return n.Name
}

func (m *model) recompute() {
	m.visible = m.visible[:0]

	// Hide the top-level root node: show its children as top level.
	for _, c := range m.root.Children {
		flatten(c, 0, &m.visible)
	}

	if m.cursor >= len(m.visible) {
		if len(m.visible) == 0 {
			m.cursor = 0
		} else {
			m.cursor = len(m.visible) - 1
		}
	}

	m.adjustScroll()
}

func (m *model) adjustScroll() {
	if m.height <= 0 {
		return
	}
	// minus header/footer space: title (2 lines) + status (2 lines)
	usable := m.height - 4
	if usable <= 0 {
		return
	}
	// Ensure cursor stays within scroll window with margins
	if m.cursor < m.scroll+minTopMargin {
		m.scroll = max(0, m.cursor-minTopMargin)
	}
	if m.cursor >= m.scroll+usable-minBottomMargin {
		m.scroll = max(0, m.cursor-usable+minBottomMargin+1)
	}
	// Clamp scroll so we don't go past end
	if m.scroll > max(0, len(m.visible)-usable) {
		m.scroll = max(0, len(m.visible)-usable)
	}
}

func flatten(n *Node, depth int, out *[]Visible) {
	*out = append(*out, Visible{N: n, Depth: depth})
	if n.IsDir && n.Expanded {
		for _, c := range n.Children {
			flatten(c, depth+1, out)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
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
				m.adjustScroll()
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}

		case "right", "l":
			if len(m.visible) == 0 {
				break
			}
			cur := m.visible[m.cursor].N
			if cur.IsDir && !cur.Expanded {
				if err := expandIfNeeded(cur); err != nil {
					m.status = "error: " + err.Error()
				} else {
					m.recompute()
				}
			}

		case "left", "h":
			if len(m.visible) == 0 {
				break
			}
			cur := m.visible[m.cursor].N
			if cur.IsDir && cur.Expanded {
				cur.Expanded = false
				m.recompute()
			}

		case "enter":
			if len(m.visible) == 0 {
				break
			}
			cur := m.visible[m.cursor].N
			if !cur.IsDir {
				// Resolve validated editor
				edPath, edArgs, err := resolveEditor()
				if err != nil {
					m.status = "editor error: " + err.Error()
					break
				}

				// Validate the file path remains inside notes root
				rootPath, err := notesRoot()
				if err != nil {
					m.status = "resolve notes root failed: " + err.Error()
					break
				}
				rel, err := filepath.Rel(rootPath, cur.Path)
				if err != nil {
					m.status = "path error: " + err.Error()
					break
				}
				safePath, err := safeJoinWithin(rootPath, rel)
				if err != nil {
					m.status = "unsafe path: " + err.Error()
					break
				}

				// Hand terminal control to the editor (TTY-safe) then resume & reload.
				cmd := exec.Command(edPath, append(edArgs, safePath)...)
				cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
				return m, tea.ExecProcess(cmd, func(error) tea.Msg { return resumedMsg{} })
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

	case resumedMsg:
		// After returning from the editor, rebuild the tree and refresh UI
		if rootPath, err := notesRoot(); err == nil {
			if root, err := buildTree(rootPath); err == nil {
				m.root = root
				m.cursor = 0
				m.recompute()
				m.status = helpText
			} else {
				m.status = "reload failed: " + err.Error()
			}
		} else {
			m.status = "resolve notes root failed: " + err.Error()
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.adjustScroll()
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

	// Render only the visible window
	usable := m.height - 4
	if usable < 1 {
		usable = len(m.visible)
	}
	end := min(len(m.visible), m.scroll+usable)

	for i := m.scroll; i < end; i++ {
		line := renderLine(m.visible[i])
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
	if v.N.IsDir {
		if v.N.Expanded {
			prefix = "▾ "
		} else {
			prefix = "▸ "
		}
	} else {
		prefix = "• "
	}
	name := displayName(v.N)
	return indent + prefix + name
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

