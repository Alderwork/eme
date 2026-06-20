package cmd

import (
	"fmt"
	"testing"

	"github.com/jinmu/eme/internal/state"
	"github.com/jinmu/eme/internal/tui"
)

func TestMaybeOnboardAgent_SetsProjectDefaultAndLaunches(t *testing.T) {
	tempState(t)
	tempCfg(t)
	stubWhich(t, "opencode")
	var target, line string
	captureSendKeys(t, &target, &line)

	prevLook := lookPath
	lookPath = func(bin string) (string, error) { return "/x/" + bin, nil }
	t.Cleanup(func() { lookPath = prevLook })
	prevPick := pickAgent
	pickAgent = func(items []tui.AgentItem, def string) (tui.AgentItem, bool, bool, error) {
		return tui.AgentItem{Name: "opencode", Command: "opencode", Installed: true}, false, false, nil
	}
	t.Cleanup(func() { pickAgent = prevPick })

	s := &state.State{Version: state.Version}
	sess := &state.Session{
		TmuxName:    "myapp",
		DisplayName: "myapp",
		Worktrees:   []state.Worktree{{Name: "main", Path: "/p/main", TmuxWindowID: "@1"}},
	}

	maybeOnboardAgent(s, sess)

	if sess.AgentCommand != "opencode" {
		t.Errorf("sess.AgentCommand = %q, want opencode", sess.AgentCommand)
	}
}

func TestMaybeOnboardAgent_NeverBlocksWhenNothingInstalled(t *testing.T) {
	tempState(t)
	tempCfg(t)
	prevLook := lookPath
	lookPath = func(bin string) (string, error) { return "", fmt.Errorf("nope") }
	t.Cleanup(func() { lookPath = prevLook })
	prevPick := pickAgent
	pickAgent = func(items []tui.AgentItem, def string) (tui.AgentItem, bool, bool, error) {
		t.Fatal("onboarding must not open the picker with no agents installed")
		return tui.AgentItem{}, false, true, nil
	}
	t.Cleanup(func() { pickAgent = prevPick })

	s := &state.State{Version: state.Version}
	sess := &state.Session{
		TmuxName:  "x",
		Worktrees: []state.Worktree{{Name: "main", TmuxWindowID: "@1"}},
	}
	maybeOnboardAgent(s, sess) // must return without calling the picker
}
