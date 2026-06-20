package reconcile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jinmu/eme/internal/git"
	"github.com/jinmu/eme/internal/runner"
	"github.com/jinmu/eme/internal/state"
	"github.com/jinmu/eme/internal/tmux"
)

// TestState_UnreachableServerDoesNotPrune guards the data-loss fix: when the
// tmux server can't be listed (e.g. not running, or pinned socket down), reconcile
// must leave state untouched instead of pruning every session — otherwise the
// caller persists an empty state and destroys records that are merely unreachable.
func TestState_UnreachableServerDoesNotPrune(t *testing.T) {
	mock := runner.NewMock()
	mock.Set("tmux", []string{"list-sessions", "-F", "#{session_name}\t#{window_id}"},
		"", "no server running", fmt.Errorf("exit status 1"))
	old := tmux.Runner
	tmux.Runner = mock
	defer func() { tmux.Runner = old }()

	s := &state.State{
		Version: state.Version,
		Sessions: []state.Session{{
			ID:        "proj-abc",
			TmuxName:  "proj",
			Worktrees: []state.Worktree{{Name: "main", TmuxWindowID: "@1"}},
		}},
	}

	if modified := State(s); modified {
		t.Fatalf("State() = true; expected no modification when server unreachable")
	}
	if len(s.Sessions) != 1 {
		t.Fatalf("session must be retained when server unreachable, got %d sessions", len(s.Sessions))
	}
}

func TestPrunablePaths(t *testing.T) {
	entries := []git.WorktreeEntry{
		{Path: "/repo", Branch: "main"},
		{Path: "/repo.worktrees/dead", Prunable: true},
		{Path: "/repo.worktrees/live", Branch: "live"},
	}
	got := prunablePaths(entries)
	if !got["/repo.worktrees/dead"] {
		t.Errorf("dead should be prunable")
	}
	if got["/repo.worktrees/live"] || got["/repo"] {
		t.Errorf("live/main must not be prunable: %v", got)
	}
}

func TestPrunablePaths_ResolvesSymlinks(t *testing.T) {
	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	linkDir := filepath.Join(base, "link")

	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("failed to create real dir: %v", err)
	}

	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	// Entry uses the symlink path
	entries := []git.WorktreeEntry{{Path: linkDir, Prunable: true}}

	// prunablePaths should resolve the symlink and key by canonical form
	got := prunablePaths(entries)

	// Verify the map is keyed by the resolved real path
	want, _ := filepath.EvalSymlinks(realDir)
	if !got[want] {
		t.Errorf("prunablePaths should contain resolved symlink path %q: %v", want, got)
	}
}
