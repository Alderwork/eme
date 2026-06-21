package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func sampleViews() []SessionView {
	return []SessionView{
		{DisplayName: "myapp", Root: "/code/myapp", Worktrees: []WorktreeView{
			{Name: "main", Branch: "main", SessionID: "myapp", IsMain: true, Status: StatusWorking, AgentLabel: "claude"},
			{Name: "feat", Branch: "feat/x", SessionID: "myapp", Status: StatusCrashed},
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

// TestDashboardRefreshReloadErrorKeepsLastKnown locks the F1 guardrail: when the
// reload (i.e. the tmux pane snapshot) fails, refresh keeps the last-known views
// verbatim and only records a transient notice — it must never blank the list or
// repaint a guessed status.
func TestDashboardRefreshReloadErrorKeepsLastKnown(t *testing.T) {
	m := NewDashboard(sampleViews(), func() ([]SessionView, error) {
		return nil, errors.New("snapshot read failed")
	})
	rowsBefore := len(m.rows) // 3 (flattened sampleViews)
	m.refresh(nil)
	if len(m.rows) != rowsBefore {
		t.Errorf("rows = %d, want %d preserved on reload error (F1 guardrail)", len(m.rows), rowsBefore)
	}
	if m.notice != "refresh failed: snapshot read failed" {
		t.Errorf("notice = %q, want the reload error surfaced", m.notice)
	}
}

// TestDashboardStickyCursorAcrossReload locks ARCH-5: when the row set reorders
// under the cursor (a session appears), the selection stays on the SAME worktree by
// identity rather than a fixed index.
func TestDashboardStickyCursorAcrossReload(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.cursor = 1 // myapp/feat
	if w := m.selected(); w == nil || w.Name != "feat" {
		t.Fatalf("precondition: cursor should be on feat, got %+v", m.selected())
	}

	// A new session appears at the top, pushing myapp/feat from index 1 to index 2.
	reordered := []SessionView{
		{DisplayName: "api", Root: "/code/api", Worktrees: []WorktreeView{
			{Name: "main", Branch: "main", SessionID: "api", IsMain: true, Status: StatusIdle},
		}},
		{DisplayName: "myapp", Root: "/code/myapp", Worktrees: []WorktreeView{
			{Name: "main", Branch: "main", SessionID: "myapp", IsMain: true, Status: StatusWorking},
			{Name: "feat", Branch: "feat/x", SessionID: "myapp", Status: StatusCrashed},
		}},
	}
	m.applyViews(reordered)

	if w := m.selected(); w == nil || w.SessionID != "myapp" || w.Name != "feat" {
		t.Errorf("cursor jumped off feat after reorder; selected = %+v", m.selected())
	}
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (feat's new index)", m.cursor)
	}
}

// TestDashboardStickyCursorFallsBackWhenSelectionGone: if the selected worktree
// disappears, the cursor falls back to a clamped, valid index (no panic).
func TestDashboardStickyCursorFallsBackWhenSelectionGone(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.cursor = 1 // myapp/feat

	m.applyViews([]SessionView{
		{DisplayName: "api", Root: "/code/api", Worktrees: []WorktreeView{
			{Name: "main", SessionID: "api", IsMain: true, Status: StatusIdle},
		}},
	})

	if m.cursor < 0 || m.cursor >= len(m.rows) {
		t.Fatalf("cursor %d out of range after selection vanished (rows=%d)", m.cursor, len(m.rows))
	}
	if w := m.selected(); w == nil {
		t.Error("selected() should be valid after fallback, got nil")
	}
}

// TestDashboardTickReloadStatusLiveDiffCarried locks the tick contract: status goes
// live from the cheap reload while the last-known git diff is carried forward, and
// the cursor stays put.
func TestDashboardTickReloadStatusLiveDiffCarried(t *testing.T) {
	initial := []SessionView{
		{DisplayName: "myapp", Root: "/code/myapp", Worktrees: []WorktreeView{
			{Name: "feat", Branch: "feat/x", SessionID: "myapp", Status: StatusIdle,
				Added: 3, Deleted: 1, HasDiff: true},
		}},
	}
	m := NewDashboard(initial, nil)
	m.cursor = 0

	// The status-only reload reports the agent now running and (as the cheap path)
	// carries NO diff of its own.
	m.SetStatusReload(func() ([]SessionView, error) {
		return []SessionView{
			{DisplayName: "myapp", Root: "/code/myapp", Worktrees: []WorktreeView{
				{Name: "feat", Branch: "feat/x", SessionID: "myapp", Status: StatusWorking},
			}},
		}, nil
	})
	m.tickReload()

	w := m.selected()
	if w == nil || w.Name != "feat" {
		t.Fatalf("cursor lost feat after tick; selected = %+v", w)
	}
	if w.Status != StatusWorking {
		t.Errorf("status = %v, want StatusWorking (live from tick)", w.Status)
	}
	if !w.HasDiff || w.Added != 3 || w.Deleted != 1 {
		t.Errorf("diff not carried: HasDiff=%v +%d -%d, want +3 -1", w.HasDiff, w.Added, w.Deleted)
	}
}

// TestDashboardTickReloadErrorKeepsLastKnown: a transient status-read failure is
// silent and preserves last-known views (F1).
func TestDashboardTickReloadErrorKeepsLastKnown(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.SetStatusReload(func() ([]SessionView, error) { return nil, errors.New("snapshot read failed") })
	rowsBefore := len(m.rows)
	statusBefore := m.views[0].Worktrees[0].Status

	m.tickReload()

	if len(m.rows) != rowsBefore {
		t.Errorf("rows = %d, want %d preserved on tick error", len(m.rows), rowsBefore)
	}
	if m.views[0].Worktrees[0].Status != statusBefore {
		t.Error("status changed on a failed tick; must keep last-known")
	}
	if m.notice != "" {
		t.Errorf("tick failure should be silent, notice = %q", m.notice)
	}
}

// TestDashboardTickReloadNilIsNoop: with no statusReload installed the tick is inert.
func TestDashboardTickReloadNilIsNoop(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	rowsBefore := len(m.rows)
	m.tickReload() // must not panic
	if len(m.rows) != rowsBefore {
		t.Errorf("rows changed on a no-op tick: %d != %d", len(m.rows), rowsBefore)
	}
}

// TestDashboardPeekToggle: `p` opens the read-only peek for the selected worktree
// and renders the captured lines; `p` again closes it.
func TestDashboardPeekToggle(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.cursor = 1 // myapp/feat
	var gotID, gotName string
	m.SetPeek(func(id, name string) ([]string, error) {
		gotID, gotName = id, name
		return []string{"building...", "done"}, nil
	})

	m.Update(runeKey('p'))
	if !m.peeking {
		t.Fatal("p should open the peek")
	}
	if gotID != "myapp" || gotName != "feat" {
		t.Errorf("peek targeted %s/%s, want myapp/feat", gotID, gotName)
	}
	if len(m.peekLines) != 2 || m.peekLines[1] != "done" {
		t.Errorf("peekLines = %v, want the captured lines", m.peekLines)
	}
	if !strings.Contains(m.View(), "done") {
		t.Error("View should show the peeked lines while peeking")
	}

	m.Update(runeKey('p'))
	if m.peeking {
		t.Error("second p should close the peek")
	}
	if strings.Contains(m.View(), "done") {
		t.Error("closed peek must spend zero rows")
	}
}

// TestDashboardPeekClosesOnMove: the peek belongs to the row it was opened on, so
// moving the cursor closes it (never a standing panel; DESIGN §5.7).
func TestDashboardPeekClosesOnMove(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.SetPeek(func(id, name string) ([]string, error) { return []string{"x"}, nil })
	m.Update(runeKey('p'))
	if !m.peeking {
		t.Fatal("precondition: peek open")
	}
	m.Update(runeKey('j'))
	if m.peeking {
		t.Error("moving down should close the peek")
	}
}

// TestDashboardPeekNilSeamIsNoop: with no peek installed, `p` is inert (no panic).
func TestDashboardPeekNilSeamIsNoop(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.Update(runeKey('p'))
	if m.peeking {
		t.Error("p with no peek seam should stay closed")
	}
}

// TestDashboardPeekErrorSurfacesNotice: a capture failure shows a transient notice
// and leaves the peek closed (never a false panel).
func TestDashboardPeekErrorSurfacesNotice(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.SetPeek(func(id, name string) ([]string, error) { return nil, errors.New("pane gone") })
	m.Update(runeKey('p'))
	if m.peeking {
		t.Error("peek should stay closed on error")
	}
	if m.notice != "peek failed: pane gone" {
		t.Errorf("notice = %q, want the peek error surfaced", m.notice)
	}
}

// TestDashboardCleanKeyRunsChildForDeadPane: `x` on a crashed worktree dispatches
// the `eme clean` child (which respawns the dead pane and clears the record).
func TestDashboardCleanKeyRunsChildForDeadPane(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.cursor = 1 // myapp/feat, StatusCrashed
	_, cmd := m.Update(runeKey('x'))
	if cmd == nil {
		t.Error("x on a crashed worktree should run the clean child")
	}
}

// TestDashboardCleanKeyNoopForLivePane: `x` is gated to dead-pane statuses, so it is
// a no-op on a running (or idle) worktree — never disturbing a live agent.
func TestDashboardCleanKeyNoopForLivePane(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	m.cursor = 0 // myapp/main, StatusWorking
	_, cmd := m.Update(runeKey('x'))
	if cmd != nil {
		t.Error("x on a running worktree should be a no-op")
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
	for _, want := range []string{"eme", "needs you", "myapp", "running", "crashed", "idle", "◐", "✗"} {
		if !strings.Contains(v, want) {
			t.Errorf("View() missing %q\n---\n%s", want, v)
		}
	}
	// One crashed worktree → "1 needs you" (clean exits no longer count).
	if !strings.Contains(v, "1 needs you") {
		t.Errorf("View() should show '1 needs you'\n%s", v)
	}
	// The dashboard is wrapped in a rounded-border panel.
	if !strings.Contains(v, "╭") || !strings.Contains(v, "╰") {
		t.Errorf("View() should be wrapped in a rounded-border panel\n%s", v)
	}
}

func TestDashboardEnterRecordsSwitchAndQuits(t *testing.T) {
	m := NewDashboard(sampleViews(), nil)
	if _, _, ok := m.SwitchTarget(); ok {
		t.Fatal("SwitchTarget should be empty before Enter")
	}
	m.cursor = 1 // myapp/feat

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	session, worktree, ok := m.SwitchTarget()
	if !ok || session != "myapp" || worktree != "feat" {
		t.Fatalf("SwitchTarget = (%q,%q,%v), want (myapp,feat,true)", session, worktree, ok)
	}
	if cmd == nil {
		t.Fatal("Enter should return a command")
	}
	// Enter must quit cleanly (so the terminal is restored before the caller
	// execs `eme switch`), not exec from inside a command.
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Error("Enter should return tea.Quit so bubbletea restores the terminal")
	}
}

// TestDashboardSelectedRowIsHighlightBar locks the headline visual: the worktree
// under the cursor renders as a full-width background highlight bar, and other
// rows do not.
func TestDashboardSelectedRowIsHighlightBar(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := NewDashboard(sampleViews(), nil)
	w := sampleViews()[0].Worktrees[0]

	selected := m.worktreeLine(w, true, 60)
	if !strings.Contains(selected, "48;2;") {
		t.Errorf("selected row should carry a background escape (highlight bar), got %q", selected)
	}
	if got := lipgloss.Width(selected); got != 60 {
		t.Errorf("selected row width = %d, want 60 (fills the inner width)", got)
	}

	plain := m.worktreeLine(w, false, 60)
	if strings.Contains(plain, "48;2;") {
		t.Errorf("non-selected row should not carry a background escape, got %q", plain)
	}
}
