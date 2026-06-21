package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jinmu/eme/internal/errors"
	"github.com/jinmu/eme/internal/state"
	"github.com/jinmu/eme/internal/tmux"
)

var cleanCmd = &cobra.Command{
	Use:   "clean <session> [worktree]",
	Short: "Clear a finished agent's dead pane, returning the worktree to idle",
	Long: `Revive the worktree's frozen pane — left behind when an exec'd agent exits or
crashes under remain-on-exit — back to a fresh shell, and clear the recorded agent so
the worktree reads idle again, ready for a new one.

It refuses while an agent is still live, so it never clears the record out from under
a running agent. The dashboard binds this to 'x' on a crashed or exited worktree.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireTmuxServer(); err != nil {
			return err
		}
		s, err := loadReconciledState()
		if err != nil {
			return err
		}
		sess, err := resolveSession(s, args[0])
		if err != nil {
			return err
		}
		worktreeName := "main"
		if len(args) == 2 {
			worktreeName = args[1]
		}
		w, err := resolveWorktree(sess, worktreeName)
		if err != nil {
			return err
		}
		if err := cleanWorktree(sess, w); err != nil {
			return err
		}
		if err := saveState(s); err != nil {
			return err
		}
		fmt.Printf("Cleaned %s/%s — pane reset to a fresh shell\n", sess.ID, w.Name)
		return nil
	},
}

// cleanWorktree revives a worktree's dead agent pane to a fresh shell and clears the
// recorded agent so status reads idle. It refuses when an agent is still live —
// clearing the record there would misreport a running agent as idle, because the
// classifier keys "alive pane + a recorded agent" as running. The respawn is
// best-effort: a dead pane revives to a shell, while an absent or already-live pane
// no-ops via the -k-less respawn error (a still-dead pane keeps reading
// crashed/exited, never a false idle).
func cleanWorktree(sess *state.Session, w *state.Worktree) error {
	running, err := agentRunningFn(w)
	if err != nil {
		return err
	}
	if running {
		return errors.New(errors.CodeCommandFailed,
			"An agent is still running in this worktree.",
			"Cleaning would clear the record while the agent is live, misreporting it as idle.",
			"Stop it first (press a), then clean.")
	}
	_ = tmux.RespawnPane(sess.TmuxName, w.TmuxWindowID, w.Path)
	w.LastAgentCommand = ""
	w.AgentPID = 0
	return nil
}
