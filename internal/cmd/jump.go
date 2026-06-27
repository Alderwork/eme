package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/alderwork/eme/internal/errors"
	"github.com/alderwork/eme/internal/state"
	"github.com/alderwork/eme/internal/tmux"
	"github.com/alderwork/eme/internal/tui"
)

const noAgentsWaitingMsg = "No agents waiting for input."

var jumpCmd = &cobra.Command{
	Use:   "jump",
	Short: "Jump to an agent waiting for your input",
	Long: `Switch the tmux client to an agent that is waiting for your input, across all
eme-managed sessions. Pair it with the waiting beacon ('eme status --tmux'): the bar
shows ●N when N agents need you, and jump lands you on one.

When several agents are waiting, jump targets the longest-waiting one; press it again
and it steps to the next (wrapping) — a stateless cycle, no daemon. When nothing is
waiting it prints a short notice and does nothing.

Waiting is detected from agent hooks, so run 'eme hooks install' first; un-hooked
agents never register as waiting. Bind it once (eme never edits your config for you):

    bind w run-shell 'eme jump'        # prefix + w`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireTmuxServer(); err != nil {
			return err
		}
		s, err := loadReconciledState()
		if err != nil {
			return err
		}
		snap, err := tmux.PanesSnapshot()
		if err != nil {
			return errors.Wrap(errors.CodeCommandFailed,
				"Could not read tmux pane state.",
				"tmux list-panes failed.",
				"Verify the tmux server is running with `tmux list-sessions`.", err)
		}
		waiting := collectWaiting(s.Sessions, snap)
		return runJumpDecision(waiting, currentWindowRef(), cmd.OutOrStdout(), func(target *waitingAgent) error {
			return switchToWorktree(s, target.session, target.worktree)
		})
	},
}

// runJumpDecision dispatches the resolved jump: pick a target via the stateless cycle and
// hand it to switchTo, or — when nothing is waiting — print the notice and exit cleanly.
// switchTo is injected so the decision is testable without a live tmux. On success it
// prints nothing, so a `run-shell` key binding never pops a message window.
func runJumpDecision(waiting []waitingAgent, current *windowRef, out io.Writer, switchTo func(*waitingAgent) error) error {
	target, ok := selectJumpTarget(waiting, current)
	if !ok {
		fmt.Fprintln(out, noAgentsWaitingMsg)
		return nil
	}
	return switchTo(target)
}

// currentWindowRef reads the window the client is currently on, for the jump cycle. A
// read failure (e.g. invoked outside an attached client) degrades to nil — jump then
// lands on the longest-waiting agent instead of erroring.
func currentWindowRef() *windowRef {
	session, windowID, err := tmux.CurrentWindow()
	if err != nil {
		return nil
	}
	return &windowRef{tmuxName: session, windowID: windowID}
}

// waitingAgent is one worktree whose agent is waiting for the user's input, paired with
// the moment it began waiting (@eme_state_at) so jump can target the longest-waiting one.
type waitingAgent struct {
	session  *state.Session
	worktree *state.Worktree
	stateAt  int64
}

// windowRef identifies the tmux window the client is currently viewing: the session
// NAME (tmux's, matched against Session.TmuxName) and the window id. A window id alone is
// ambiguous — ids repeat across sessions — so the current-match keys on both.
type windowRef struct {
	tmuxName string
	windowID string
}

// collectWaiting returns every worktree whose agent is waiting for input, read from the
// batched pane snapshot via the same classifier the dashboard uses. Only hooked agents
// ever classify as waiting; everything else (working/idle/crashed/exited) is skipped.
func collectWaiting(sessions []state.Session, snap map[string]tmux.PaneInfo) []waitingAgent {
	var waiting []waitingAgent
	for i := range sessions {
		s := &sessions[i]
		for j := range s.Worktrees {
			w := &s.Worktrees[j]
			info, present := snap[w.TmuxWindowID]
			if classifyStatus(info, present, w.LastAgentCommand) == tui.StatusWaiting {
				waiting = append(waiting, waitingAgent{session: s, worktree: w, stateAt: info.EmeStateAt})
			}
		}
	}
	return waiting
}

// selectJumpTarget picks which waiting agent to jump to, implementing the stateless
// cycle: order by longest-waiting first (stateAt ascending; ties broken by session name
// then window id for a stable order), then — if the client is already sitting on a
// waiting agent — return the NEXT one (wrapping), else return the oldest. ok is false
// only when nothing is waiting. It never mutates its input (it sorts a copy).
func selectJumpTarget(waiting []waitingAgent, current *windowRef) (*waitingAgent, bool) {
	if len(waiting) == 0 {
		return nil, false
	}
	ordered := make([]waitingAgent, len(waiting))
	copy(ordered, waiting)
	sort.SliceStable(ordered, func(a, b int) bool {
		if ordered[a].stateAt != ordered[b].stateAt {
			return ordered[a].stateAt < ordered[b].stateAt
		}
		if ordered[a].session.TmuxName != ordered[b].session.TmuxName {
			return ordered[a].session.TmuxName < ordered[b].session.TmuxName
		}
		return ordered[a].worktree.TmuxWindowID < ordered[b].worktree.TmuxWindowID
	})

	start := 0
	if current != nil {
		for i := range ordered {
			if ordered[i].session.TmuxName == current.tmuxName &&
				ordered[i].worktree.TmuxWindowID == current.windowID {
				start = (i + 1) % len(ordered)
				break
			}
		}
	}
	return &ordered[start], true
}
