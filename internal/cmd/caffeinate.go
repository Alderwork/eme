package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/jinmu/eme/internal/errors"
	"github.com/jinmu/eme/internal/state"
	"github.com/jinmu/eme/internal/tmux"
	"github.com/jinmu/eme/internal/tui"
)

// caffeinateWindowName is the hidden tmux window that hosts a session's caffeinate
// daemon. Detection and teardown key off this exact name.
const caffeinateWindowName = "__eme_caffeinate"

// caffeinatePollInterval is how often auto-mode re-samples the session's agents.
const caffeinatePollInterval = 3 * time.Second

// caffeinateSupportedFn reports whether this platform supports caffeinate. A var
// seam so tests can force it on regardless of the host OS.
var caffeinateSupportedFn = func() bool { return runtime.GOOS == "darwin" }

// anyWorking reports whether any of the session's panes classify as a working agent.
func anyWorking(statuses []tui.AgentStatus) bool {
	for _, s := range statuses {
		if s == tui.StatusWorking {
			return true
		}
	}
	return false
}

// shouldAssert is the pure auto-mode decision: hold caffeinate while an agent is
// working, or for `grace` after the last working sample. Manual mode never calls
// this (it asserts unconditionally).
func shouldAssert(working bool, sinceLastWorking, grace time.Duration) bool {
	return working || sinceLastWorking < grace
}

// normalizeMode validates/normalizes a --mode value to off|manual|auto.
func normalizeMode(m string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "off":
		return "off", nil
	case "manual":
		return "manual", nil
	case "auto":
		return "auto", nil
	default:
		return "", errors.New(errors.CodeCommandFailed,
			fmt.Sprintf("invalid caffeinate mode %q.", m),
			"Mode must be one of: off, manual, auto.",
			"Run `eme caffeinate <session> --mode manual` (or auto/off).")
	}
}

// emeExecutable resolves the running eme binary's absolute path. A seam for tests.
var emeExecutable = os.Executable

// armCaffeinate (re)starts the session's hidden caffeinate window in the given mode.
// It first disarms any existing window so a mode change takes effect, then spawns a
// fresh daemon by absolute eme path (PATH-independent). Bound to the tmux session:
// when the session dies the window dies and the daemon's caffeinate child dies with
// it. No-op off macOS.
func armCaffeinate(sess *state.Session, mode string) error {
	if !caffeinateSupportedFn() {
		return nil
	}
	_ = disarmCaffeinate(sess) // drop any stale/previous-mode window first (best-effort)
	bin, err := emeExecutable()
	if err != nil {
		return errors.New(errors.CodeCommandFailed,
			"could not locate the eme binary to start caffeinate.",
			err.Error(),
			"Reinstall eme or report this if it persists.")
	}
	if _, err := tmux.NewWindowCmd(sess.TmuxName, caffeinateWindowName, sess.MainPath(),
		bin, "caffeinate-daemon", sess.ID, "--mode", mode); err != nil {
		return errors.New(errors.CodeCommandFailed,
			"could not start the caffeinate window.",
			err.Error(),
			"Make sure the session's tmux server is reachable.")
	}
	return nil
}

// disarmCaffeinate kills the session's caffeinate window by name (best-effort: a
// missing window is fine). Killing the window SIGHUPs the daemon + its caffeinate
// child, releasing the assertion.
func disarmCaffeinate(sess *state.Session) error {
	return tmux.KillWindow(sess.TmuxName, caffeinateWindowName)
}

// setCaffeinate applies a normalized mode (off|manual|auto) to a session: it arms or
// disarms the window FIRST (so persisted intent never claims a state tmux isn't in),
// then records the intent and saves. off → "" intent.
func setCaffeinate(s *state.State, sess *state.Session, mode string) error {
	switch mode {
	case "off":
		_ = disarmCaffeinate(sess)
		sess.CaffeinateMode = ""
	case "manual", "auto":
		if err := armCaffeinate(sess, mode); err != nil {
			return err
		}
		sess.CaffeinateMode = mode
	}
	return saveState(s)
}
