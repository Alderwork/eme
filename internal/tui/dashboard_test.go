package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func sampleViews() []SessionView {
	return []SessionView{
		{DisplayName: "myapp", Root: "/code/myapp", Worktrees: []WorktreeView{
			{Name: "main", Branch: "main", SessionID: "myapp", IsMain: true, Status: StatusWorking, AgentLabel: "claude"},
			{Name: "feat", Branch: "feat/x", SessionID: "myapp", Status: StatusExited},
		}},
		{DisplayName: "api", Root: "/code/api", Worktrees: []WorktreeView{
			{Name: "main", Branch: "main", SessionID: "api", IsMain: true, Status: StatusIdle},
		}},
	}
}

func TestDashboardFlattenAndCursorClamp(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	if len(m.rows) != 3 {
		t.Fatalf("rows = %d, want 3 (flattened worktrees)", len(m.rows))
	}
	for i := 0; i < 10; i++ {
		m.Update(runeKey('j'))
	}
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want clamped at 2", m.cursor)
	}
}

func TestDashboardKillContext_MainKillsSession(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.cursor = 0 // myapp/main (IsMain)
	m.Update(runeKey('d'))
	if m.pending == nil || !m.pending.isMain || m.pending.sessionID != "myapp" {
		t.Fatalf("pending = %+v, want isMain session kill of myapp", m.pending)
	}
	_, cmd := m.Update(runeKey('y'))
	if cmd == nil {
		t.Error("confirming kill should return a command")
	}
	if m.pending != nil {
		t.Error("pending should clear after confirm")
	}
}

func TestDashboardKillContext_WorktreeKill(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.cursor = 1 // myapp/feat (not main)
	m.Update(runeKey('d'))
	if m.pending == nil || m.pending.isMain || m.pending.worktreeName != "feat" {
		t.Fatalf("pending = %+v, want worktree kill of feat", m.pending)
	}
	_, cmd := m.Update(runeKey('n')) // cancel
	if cmd != nil || m.pending != nil {
		t.Error("cancel should clear pending and return no command")
	}
}

func TestDashboardRefreshRebuildsRows(t *testing.T) {
	m := NewDashboard(sampleViews(), func() ([]SessionView, error) {
		return []SessionView{{DisplayName: "api", Root: "/code/api", Worktrees: []WorktreeView{
			{Name: "main", SessionID: "api", IsMain: true, Status: StatusIdle},
		}}}, nil
	})
	m.cursor = 2
	m.refresh(nil)
	if len(m.rows) != 1 {
		t.Fatalf("rows = %d, want 1 after refresh", len(m.rows))
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want clamped to 0", m.cursor)
	}
}

func TestDashboardRefreshActionErrorIsTransient(t *testing.T) {
	m := NewDashboard(sampleViews(), func() ([]SessionView, error) { return sampleViews(), nil })
	m.refresh(errors.New("kill failed"))
	if m.notice != "kill failed" {
		t.Errorf("notice = %q, want the action error", m.notice)
	}
	if len(m.rows) != 3 {
		t.Errorf("rows = %d, want list preserved", len(m.rows))
	}
}

func TestDashboardViewContainsMotifAndStatus(t *testing.T) {
	v := NewDashboard(sampleViews(), nil).View()
	for _, want := range []string{"eme", "needs you", "myapp", "working", "exited", "idle", "◐", "○"} {
		if !strings.Contains(v, want) {
			t.Errorf("View() missing %q\n---\n%s", want, v)
		}
	}
	// One exited worktree → "1 needs you".
	if !strings.Contains(v, "1 needs you") {
		t.Errorf("View() should show '1 needs you'\n%s", v)
	}
}
