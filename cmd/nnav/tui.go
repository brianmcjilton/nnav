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

// Short, discoverable key map displayed in the status/footer.
// Keep this in sync with Update() to avoid confusing users.
const helpText = "↑/↓ move • → expand • ← collapse • <enter> open • <q> quit"

// Soft viewport margins so the cursor isn't pinned to the edges while scrolling.
const minTopMargin = 2    // lines to keep above cursor
const minBottomMargin = 2 // lines to keep below cursor

// Visible represents a flattened view of the tree for rendering and navigation.
// Depth is used to indent items visually in the list.
type Visible struct {
	N     *Node
	Depth int
}

// model is the Bubble Tea state container for the TUI.
// - root: the logical tree (directories + files).
// - visible: a flattened, scrollable projection of the expanded nodes.
// - cursor: index into visible for the current selection.
// - scroll: top index of the viewport window within visible.
// - status: footer text for help/errors.
// - width/height: last-known terminal dimensions used for layout.
type model struct {
	root       *Node
	cursor     int
	visible    []Visible
	status     string
	width      int
	height     int
	scroll     int // top index of visible window
	searchTerm string
}

// message sent after we return from the editor
// Used to trigger a post-editor refresh without coupling to exec exit codes.
type resumedMsg struct{}

// newModel initializes the model and precomputes the initial visible list.
// Starts with the root expanded at top-level.
func newModel(root *Node, term string) model {
	m := model{root: root, cursor: 0, status: helpText, searchTerm: term}
	m.recompute()
	return m
}

// displayName returns what we render for a node:
// - files: Title if present, else filename
// - dirs: directory name
// Prefer a human-friendly title to make scanning large lists easier.
func displayName(n *Node) string {
	if n.IsDir {
		return n.Name
	}
	if t := strings.TrimSpace(n.Title); t != "" {
		return t
	}
	return n.Name
}

// recompute rebuilds the flattened visible list from the current tree state.
// It hides the synthetic root and shows its children as the top level.
// Keeps the cursor within bounds after recomputation.
func (m *model) recompute() {
	m.visible = m.visible[:0]

	// Hide the top-level root node: show its children as top level.
	for _, c := range m.root.Children {
		flatten(c, 0, &m.visible)
	}

	// Clamp cursor within new bounds.
	if m.cursor >= len(m.visible) {
		if len(m.visible) == 0 {
			m.cursor = 0
		} else {
			m.cursor = len(m.visible) - 1
		}
	}

	m.adjustScroll()
}

// adjustScroll ensures the viewport scroll offset includes the cursor with margins.
// Reserves space for a 2-line title and 2-line footer/status (total header/footer = 4).
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

// flatten appends n and (recursively) its expanded children to out,
// tracking depth for indentation in the renderer.
// This is the authoritative projection used by navigation and rendering.
func flatten(n *Node, depth int, out *[]Visible) {
	*out = append(*out, Visible{N: n, Depth: depth})
	if n.IsDir && n.Expanded {
		for _, c := range n.Children {
			flatten(c, depth+1, out)
		}
	}
}

// Simple helpers (avoid pulling additional deps just for min/max).
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

// Init implements Bubble Tea’s initializer—no async startup work needed.
func (m model) Init() tea.Cmd { return nil }

// Update is the event loop: handles key presses, window resizes, and editor resume.
// All state mutations funnel through here for predictable TUI behavior.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {

		case "q", "esc", "ctrl+c":
			// Exit cleanly from the alt screen back to the user's terminal.
			return m, tea.Quit

		case "down", "j":
			// Cursor moves down within the visible list.
			if m.cursor < len(m.visible)-1 {
				m.cursor++
				m.adjustScroll()
			}

		case "up", "k":
			// Cursor moves up within the visible list.
			if m.cursor > 0 {
				m.cursor--
				m.adjustScroll()
			}

		case "right", "l":
			// Expand directory at the cursor (lazy-load children if needed).
			if len(m.visible) == 0 {
				break
			}
			cur := m.visible[m.cursor].N
			if cur.IsDir && !cur.Expanded {
				if err := expandIfNeeded(cur, m.searchTerm); err != nil {
					m.status = "error: " + err.Error()
				} else {
					m.recompute()
				}
			}

		case "left", "h":
			// Collapse directory at the cursor.
			if len(m.visible) == 0 {
				break
			}
			cur := m.visible[m.cursor].N
			if cur.IsDir && cur.Expanded {
				cur.Expanded = false
				m.recompute()
			}

		case "enter":
			// Open the selected file in a validated, allowlisted editor.
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

				// Validate the file path remains inside notes root (defense-in-depth).
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

				// Hand terminal control to the editor with TTY attached.
				// tea.ExecProcess returns control to Bubble Tea and sends resumedMsg when done.
				cmd := exec.Command(edPath, append(edArgs, safePath)...)
				cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
				return m, tea.ExecProcess(cmd, func(error) tea.Msg { return resumedMsg{} })
			}

		case "r":
			// Manual refresh: rebuild the tree from disk and reset view state.
			// Useful when files are added/removed externally.
			rootPath, _ := notesRoot()
			if root, err := buildTree(rootPath, m.searchTerm); err == nil {
				m.root = root
				m.cursor = 0
				m.recompute()
				m.status = "reloaded at " + time.Now().Format("15:04:05")
			} else {
				m.status = "reload failed: " + err.Error()
			}
		}

	case resumedMsg:
		// After returning from the editor, rebuild tree and reset the help footer.
		// This ensures titles/ordering reflect any edits or renames.
		if rootPath, err := notesRoot(); err == nil {
			if root, err := buildTree(rootPath, m.searchTerm); err == nil {
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
		// Track terminal size for layout and scrolling calculations.
		m.width, m.height = msg.Width, msg.Height
		m.adjustScroll()
	}

	return m, nil
}

// View renders the current screen using lipgloss styles.
// Layout: title (2 lines), list (scrollable window), status/footer (2 lines).
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
			// Visual cursor: reverse video for strong affordance.
			line = cursorStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Footer/status line with help or error messages.
	b.WriteString("\n")
	b.WriteString(muted.Render(m.status))
	b.WriteString("\n")
	return b.String()
}

// renderLine draws a single entry with indentation and a prefix glyph:
// - ▸/▾ for directories (collapsed/expanded), • for files.
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

// expandIfNeeded lazily loads children for a directory if not already populated,
// and marks it expanded. No-op for files or already-expanded dirs.
func expandIfNeeded(n *Node, term string) error {
	if !n.IsDir {
		return nil
	}
	if n.Expanded && len(n.Children) > 0 {
		return nil
	}
	if len(n.Children) == 0 {
		kids, err := readDirNodes(n.Path, term)
		if err != nil {
			return err
		}
		n.Children = kids
	}
	n.Expanded = true
	return nil
}

