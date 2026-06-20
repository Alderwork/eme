// Package tui implements the eme terminal user interface.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// titleStyle, cursorStyle, mutedStyle, errorStyle, helpStyle are SHARED with
// picker.go / input.go (same package) and MUST remain defined here — do not drop
// them when rewriting this file.
var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	rhymeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	needsYouStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5C07B"))
	sessionStyle  = lipgloss.NewStyle().Bold(true)
	rootStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	branchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	addStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	delStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
)

// rowRef points at a worktree within the view-model.
type rowRef struct{ session, worktree int }

// killTarget describes a pending kill confirmation.
type killTarget struct {
	sessionID    string
	worktreeName string
	label        string
	isMain       bool
}

// DashboardModel is the main dashboard.
type DashboardModel struct {
	views    []SessionView
	rows     []rowRef // flattened selectable worktree rows, in render order
	cursor   int      // index into rows
	width    int
	height   int
	notice   string
	pending  *killTarget
	showHelp bool
	// reload re-reads the view-model after a child action returns. May be nil
	// (tests), in which case the list is not refreshed.
	reload func() ([]SessionView, error)
}

// NewDashboard creates a dashboard model. reload is called after each child
// action (create/kill/agent) completes to refresh the view-model.
func NewDashboard(views []SessionView, reload func() ([]SessionView, error)) *DashboardModel {
	m := &DashboardModel{views: views, reload: reload}
	m.rebuildRows()
	return m
}

// rebuildRows recomputes the flattened selectable list and clamps the cursor.
func (m *DashboardModel) rebuildRows() {
	m.rows = nil
	for si := range m.views {
		for wi := range m.views[si].Worktrees {
			m.rows = append(m.rows, rowRef{session: si, worktree: wi})
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// selected returns the worktree under the cursor, or nil if the list is empty.
func (m *DashboardModel) selected() *WorktreeView {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	r := m.rows[m.cursor]
	return &m.views[r.session].Worktrees[r.worktree]
}

// needsYouCount counts worktrees whose status warrants attention.
func (m *DashboardModel) needsYouCount() int {
	n := 0
	for si := range m.views {
		for _, w := range m.views[si].Worktrees {
			if w.Status.NeedsAttention() {
				n++
			}
		}
	}
	return n
}

// actionFinishedMsg is delivered after a child `eme` process exits.
type actionFinishedMsg struct{ err error }

// Init implements tea.Model.
func (m *DashboardModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.pending != nil {
			t := m.pending
			m.pending = nil
			if msg.String() == "y" {
				if t.isMain {
					return m, m.runChild("kill", t.sessionID, "--force")
				}
				return m, m.runChild("kill", t.sessionID, t.worktreeName, "--force")
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "enter", "o":
			if w := m.selected(); w != nil {
				return m, m.switchTo(w.SessionID, w.Name)
			}
		case "n":
			return m, m.runChild("new", "--no-switch")
		case "c":
			if w := m.selected(); w != nil {
				return m, m.runChild("new", "--worktree", w.SessionID, "--no-switch")
			}
		case "a":
			if w := m.selected(); w != nil {
				return m, m.runChild("agent", w.SessionID, w.Name)
			}
		case "d":
			if w := m.selected(); w != nil {
				t := &killTarget{sessionID: w.SessionID, worktreeName: w.Name, isMain: w.IsMain}
				if w.IsMain {
					t.label = "project " + m.views[m.rows[m.cursor].session].DisplayName
				} else {
					t.label = "worktree " + w.Name
				}
				m.pending = t
				m.notice = ""
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case actionFinishedMsg:
		m.refresh(msg.err)
	}
	return m, nil
}

// View implements tea.Model.
func (m *DashboardModel) View() string {
	var b strings.Builder

	left := titleStyle.Render("╶╶ eme ╶╶") + "  " + rhymeStyle.Render("eeny · meeny · miny · moe")
	right := ""
	if n := m.needsYouCount(); n > 0 {
		right = needsYouStyle.Render(fmt.Sprintf("%d needs you", n))
	}
	b.WriteString(m.headerLine(left, right))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(mutedStyle.Render("No sessions. Press 'n' to create one.") + "\n")
	} else {
		rowi := 0
		for si := range m.views {
			sv := m.views[si]
			b.WriteString(fmt.Sprintf(" %d  %s  %s\n", si+1, sessionStyle.Render(sv.DisplayName), rootStyle.Render(sv.Root)))
			for wi := range sv.Worktrees {
				w := sv.Worktrees[wi]
				marker := "  "
				nameCell := fmt.Sprintf("%-14s", w.Name)
				if rowi == m.cursor {
					marker = cursorStyle.Render("▸ ")
					nameCell = cursorStyle.Render(nameCell)
				}
				status := statusStyle[w.Status].Render(w.Status.Glyph() + " " + w.Status.Label())
				trailer := w.AgentLabel
				if trailer == "" && w.HasDiff {
					trailer = addStyle.Render(fmt.Sprintf("+%d", w.Added)) + " " + delStyle.Render(fmt.Sprintf("-%d", w.Deleted))
				}
				b.WriteString(fmt.Sprintf("  %s%s %s  %s  %s\n",
					marker, nameCell, branchStyle.Render(fmt.Sprintf("%-16s", w.Branch)), status, trailer))
				rowi++
			}
			b.WriteString("\n")
		}
	}

	if m.pending != nil {
		b.WriteString(errorStyle.Render("kill "+m.pending.label+"?  y = confirm, any other key = cancel") + "\n")
	} else if m.notice != "" {
		b.WriteString(errorStyle.Render(m.notice) + "\n")
	}

	if m.showHelp {
		b.WriteString(helpStyle.Render("n new · c worktree · a agent · ↵/o open · d kill · q quit · ?") + "\n")
	} else {
		b.WriteString(helpStyle.Render("?: help") + "\n")
	}
	return b.String()
}

// headerLine right-aligns right against the known width, falling back to a small
// gap when width is unknown.
func (m *DashboardModel) headerLine(left, right string) string {
	if right == "" {
		return left
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

// refresh re-reads the view-model after a child action, recording any error as a
// transient notice. It never quits the dashboard.
func (m *DashboardModel) refresh(actionErr error) {
	if actionErr != nil {
		m.notice = actionErr.Error()
	} else {
		m.notice = ""
	}
	if m.reload == nil {
		return
	}
	views, err := m.reload()
	if err != nil {
		m.notice = "refresh failed: " + err.Error()
		return
	}
	m.views = views
	m.rebuildRows()
}

// runChild runs `eme <args...>` as a child process, pausing the dashboard and
// handing it the terminal, then resumes and refreshes.
func (m *DashboardModel) runChild(args ...string) tea.Cmd {
	binary, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return actionFinishedMsg{err: fmt.Errorf("locate eme binary: %w", err)} }
	}
	return tea.ExecProcess(exec.Command(binary, args...), func(err error) tea.Msg {
		return actionFinishedMsg{err: err}
	})
}

// switchTo replaces this process with `eme switch <session> <worktree>`, leaving
// the dashboard. On success it never returns.
func (m *DashboardModel) switchTo(sessionID, worktree string) tea.Cmd {
	return func() tea.Msg {
		binary, err := os.Executable()
		if err != nil {
			return actionFinishedMsg{err: fmt.Errorf("locate eme binary: %w", err)}
		}
		err = syscall.Exec(binary, []string{"eme", "switch", sessionID, worktree}, os.Environ())
		return actionFinishedMsg{err: fmt.Errorf("exec eme switch: %w", err)}
	}
}
