package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alderwork/eme/internal/state"
	"github.com/alderwork/eme/internal/tmux"
)

// mkWaiting builds a minimal waitingAgent for selection tests — only the fields the
// ordering and current-match logic read (tmux session name, window id, stateAt).
func mkWaiting(tmuxName, windowID string, stateAt int64) waitingAgent {
	return waitingAgent{
		session:  &state.Session{TmuxName: tmuxName},
		worktree: &state.Worktree{TmuxWindowID: windowID},
		stateAt:  stateAt,
	}
}

// TestSelectJumpTarget_Empty: nothing waiting → no target.
func TestSelectJumpTarget_Empty(t *testing.T) {
	if _, ok := selectJumpTarget(nil, nil); ok {
		t.Fatal("empty waiting set → want ok=false")
	}
}

// TestSelectJumpTarget_OldestFirst: with no current window, jump lands on the
// longest-waiting agent (smallest stateAt), regardless of input order.
func TestSelectJumpTarget_OldestFirst(t *testing.T) {
	waiting := []waitingAgent{
		mkWaiting("proj", "@9", 300),
		mkWaiting("proj", "@2", 100), // oldest
		mkWaiting("proj", "@5", 200),
	}
	got, ok := selectJumpTarget(waiting, nil)
	if !ok || got.worktree.TmuxWindowID != "@2" {
		t.Fatalf("no current → want oldest @2, got ok=%v win=%v", ok, win(got))
	}
}

// TestSelectJumpTarget_CycleToNext: when the client is already on the oldest waiting
// agent, jump steps to the next one in order.
func TestSelectJumpTarget_CycleToNext(t *testing.T) {
	waiting := []waitingAgent{
		mkWaiting("proj", "@2", 100), // oldest
		mkWaiting("proj", "@5", 200),
		mkWaiting("proj", "@9", 300),
	}
	cur := &windowRef{tmuxName: "proj", windowID: "@2"}
	got, ok := selectJumpTarget(waiting, cur)
	if !ok || got.worktree.TmuxWindowID != "@5" {
		t.Fatalf("on oldest → want next @5, got ok=%v win=%v", ok, win(got))
	}
}

// TestSelectJumpTarget_WrapsAround: on the last waiting agent, the next press wraps
// back to the oldest.
func TestSelectJumpTarget_WrapsAround(t *testing.T) {
	waiting := []waitingAgent{
		mkWaiting("proj", "@2", 100),
		mkWaiting("proj", "@5", 200),
		mkWaiting("proj", "@9", 300), // last
	}
	cur := &windowRef{tmuxName: "proj", windowID: "@9"}
	got, ok := selectJumpTarget(waiting, cur)
	if !ok || got.worktree.TmuxWindowID != "@2" {
		t.Fatalf("on last → want wrap to @2, got ok=%v win=%v", ok, win(got))
	}
}

// TestSelectJumpTarget_CurrentNotWaiting: when the client's window is not itself a
// waiting agent (e.g. you're on a working agent), jump goes to the oldest waiting.
func TestSelectJumpTarget_CurrentNotWaiting(t *testing.T) {
	waiting := []waitingAgent{
		mkWaiting("proj", "@5", 200),
		mkWaiting("proj", "@2", 100),
	}
	cur := &windowRef{tmuxName: "proj", windowID: "@99"} // not in the set
	got, ok := selectJumpTarget(waiting, cur)
	if !ok || got.worktree.TmuxWindowID != "@2" {
		t.Fatalf("current not waiting → want oldest @2, got ok=%v win=%v", ok, win(got))
	}
}

// TestSelectJumpTarget_CrossSessionMatch: a window id can repeat across tmux servers,
// so the current-match must key on (session, window), not window alone. Here @2 exists
// in both sessions; being on web:@2 must cycle within the ordered set, not mismatch.
func TestSelectJumpTarget_CrossSessionMatch(t *testing.T) {
	waiting := []waitingAgent{
		mkWaiting("web", "@2", 100), // oldest, web
		mkWaiting("api", "@2", 200), // same window id, different session
	}
	cur := &windowRef{tmuxName: "web", windowID: "@2"}
	got, ok := selectJumpTarget(waiting, cur)
	if !ok || got.session.TmuxName != "api" || got.worktree.TmuxWindowID != "@2" {
		t.Fatalf("on web:@2 → want next api:@2, got ok=%v %s:%s", ok, got.session.TmuxName, win(got))
	}
}

// TestSelectJumpTarget_StableTiebreak: equal stateAt orders deterministically by
// (tmux session name, window id) so repeated cycling is predictable.
func TestSelectJumpTarget_StableTiebreak(t *testing.T) {
	waiting := []waitingAgent{
		mkWaiting("proj", "@9", 100),
		mkWaiting("proj", "@3", 100), // same age, lower window id → first
	}
	got, ok := selectJumpTarget(waiting, nil)
	if !ok || got.worktree.TmuxWindowID != "@3" {
		t.Fatalf("equal age → want @3 by tiebreak, got ok=%v win=%v", ok, win(got))
	}
}

// TestCollectWaiting selects exactly the StatusWaiting worktrees from a snapshot —
// hooked waiting agents only — and carries each one's @eme_state_at.
func TestCollectWaiting(t *testing.T) {
	sessions := []state.Session{{
		TmuxName: "proj", DisplayName: "proj",
		Worktrees: []state.Worktree{
			{Name: "feat-a", TmuxWindowID: "@1", LastAgentCommand: "claude"}, // waiting
			{Name: "feat-b", TmuxWindowID: "@2", LastAgentCommand: "claude"}, // working
			{Name: "main", TmuxWindowID: "@3", LastAgentCommand: "claude"},   // idle
			{Name: "feat-c", TmuxWindowID: "@4", LastAgentCommand: "claude"}, // waiting (older)
			{Name: "gone", TmuxWindowID: "@5", LastAgentCommand: "claude"},   // absent → exited
			{Name: "boom", TmuxWindowID: "@6", LastAgentCommand: "claude"},   // crashed
		},
	}}
	snap := map[string]tmux.PaneInfo{
		"@1": {Command: "node", EmeState: "waiting", EmeStateAt: 100},
		"@2": {Command: "node"}, // working
		"@3": {Command: "zsh"},  // idle
		"@4": {Command: "node", EmeState: "waiting", EmeStateAt: 50},
		// @5 deliberately absent from the snapshot (→ exited)
		// A dead pane never classifies as waiting even with a stale @eme_state — this is the
		// headline of the waiting-only segment: crashes are excluded, not jumped to.
		"@6": {Dead: true, DeadStatus: 1, EmeState: "waiting", EmeStateAt: 10},
	}

	got := collectWaiting(sessions, snap)
	if len(got) != 2 {
		t.Fatalf("want 2 waiting agents, got %d", len(got))
	}
	byWin := map[string]int64{}
	for _, a := range got {
		byWin[a.worktree.TmuxWindowID] = a.stateAt
	}
	if byWin["@1"] != 100 || byWin["@4"] != 50 {
		t.Fatalf("waiting set/stateAt wrong: %+v", byWin)
	}
}

// TestRunJumpDecision_NoWaiting: with nothing waiting, jump prints the friendly notice,
// never switches, and exits without error (so a key binding shows a message, not a crash).
func TestRunJumpDecision_NoWaiting(t *testing.T) {
	var buf bytes.Buffer
	switched := false
	err := runJumpDecision(nil, nil, &buf, func(*waitingAgent) error { switched = true; return nil })
	if err != nil {
		t.Fatalf("no waiting → unexpected error: %v", err)
	}
	if switched {
		t.Fatal("no waiting → must not switch")
	}
	if !strings.Contains(buf.String(), "No agents waiting") {
		t.Fatalf("want a no-waiting notice, got %q", buf.String())
	}
}

// TestRunJumpDecision_SwitchesToTarget: with an agent waiting, jump switches to the
// selected target and prints nothing (so a run-shell binding pops no message window).
func TestRunJumpDecision_SwitchesToTarget(t *testing.T) {
	waiting := []waitingAgent{mkWaiting("proj", "@2", 100)}
	var got *waitingAgent
	var buf bytes.Buffer
	err := runJumpDecision(waiting, nil, &buf, func(a *waitingAgent) error { got = a; return nil })
	if err != nil {
		t.Fatalf("switch → unexpected error: %v", err)
	}
	if got == nil || got.worktree.TmuxWindowID != "@2" {
		t.Fatalf("want switch to @2, got %v", win(got))
	}
	if buf.String() != "" {
		t.Fatalf("switch path must print nothing, got %q", buf.String())
	}
}

func win(a *waitingAgent) string {
	if a == nil {
		return "<nil>"
	}
	return a.worktree.TmuxWindowID
}
