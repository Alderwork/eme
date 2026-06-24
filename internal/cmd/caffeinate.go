package cmd

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/jinmu/eme/internal/errors"
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
