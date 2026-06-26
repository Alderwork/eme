package tui

import "testing"

// assertOnSession fails unless the cursor rests on session wantSI's header row.
func assertOnSession(t *testing.T, m *DashboardModel, wantSI int) {
	t.Helper()
	r := m.currentRow()
	if r == nil || r.kind != rowSession || r.session != wantSI {
		t.Fatalf("cursor not on project %d header: row=%+v cursor=%d", wantSI, r, m.cursor)
	}
}

// TestNumberKeyJumpsToProject: pressing the ordinal shown in a project's header jumps the
// cursor straight to that project, from anywhere; an out-of-range number is a no-op.
func TestNumberKeyJumpsToProject(t *testing.T) {
	m := NewDashboard(manyViews(3, 2), nil) // projects 1..3, each with 2 worktrees

	m.Update(runeKey('3'))
	assertOnSession(t, m, 2) // ordinal 3 → session index 2

	m.Update(runeKey('1'))
	assertOnSession(t, m, 0)

	// Works even when the cursor is down on a worktree row.
	m.cursor = 1 // a worktree under project 1
	m.Update(runeKey('2'))
	assertOnSession(t, m, 1)

	// A number with no matching project leaves the cursor put — never a surprise jump.
	before := m.cursor
	m.Update(runeKey('9'))
	if m.cursor != before {
		t.Fatalf("9 with only 3 projects must be a no-op; cursor moved %d → %d", before, m.cursor)
	}
}

// TestBracketKeysStepBetweenProjects: [ and ] move to the previous/next project header and
// clamp (no wrap) at the ends.
func TestBracketKeysStepBetweenProjects(t *testing.T) {
	m := NewDashboard(manyViews(3, 2), nil)
	assertOnSession(t, m, 0)

	m.Update(runeKey(']'))
	assertOnSession(t, m, 1)
	m.Update(runeKey(']'))
	assertOnSession(t, m, 2)
	m.Update(runeKey(']')) // clamp at the last project
	assertOnSession(t, m, 2)

	m.Update(runeKey('['))
	assertOnSession(t, m, 1)
	m.Update(runeKey('['))
	assertOnSession(t, m, 0)
	m.Update(runeKey('[')) // clamp at the first project
	assertOnSession(t, m, 0)
}
