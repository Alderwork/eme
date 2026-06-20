package cmd

import (
	"fmt"
	"os"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinmu/eme/internal/state"
	"github.com/jinmu/eme/internal/tui"
)

func runDashboard() error {
	s, err := loadReconciledState()
	if err != nil {
		return err
	}

	// If stdout is not a terminal, we cannot open an interactive TUI.
	if !isTerminal() {
		return printSessionList(s)
	}

	model := tui.NewDashboard(buildSessionViews(s.Sessions), func() ([]tui.SessionView, error) {
		rs, err := loadReconciledState()
		if err != nil {
			return nil, err
		}
		return buildSessionViews(rs.Sessions), nil
	})
	finalModel, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}
	return switchFromModel(finalModel)
}

// switchFromModel execs `eme switch` if the dashboard recorded a switch target
// (Enter), otherwise returns nil. It runs after bubbletea has restored the
// terminal. Split out from runDashboard so the cross-package handoff is testable
// without a TTY.
func switchFromModel(finalModel tea.Model) error {
	dm, ok := finalModel.(*tui.DashboardModel)
	if !ok {
		return nil
	}
	session, worktree, ok := dm.SwitchTarget()
	if !ok {
		return nil
	}
	return execSwitch(session, worktree)
}

// execReplace replaces the current process image. It is a package var so tests
// can capture the argv without actually exec'ing.
var execReplace = syscall.Exec

// execSwitch replaces the current process with `eme switch <session> [worktree]`.
// It runs only after the dashboard's bubbletea program has exited and restored
// the terminal, so the handoff happens on a clean terminal.
func execSwitch(session, worktree string) error {
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate eme binary: %w", err)
	}
	argv := []string{"eme", "switch", session}
	if worktree != "" {
		argv = append(argv, worktree)
	}
	return execReplace(binary, argv, os.Environ())
}

func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) == os.ModeCharDevice
}

func printSessionList(s *state.State) error {
	if len(s.Sessions) == 0 {
		fmt.Println("No sessions. Run `eme new` to create one.")
		return nil
	}
	for _, sess := range s.Sessions {
		fmt.Printf("%s  %s\n", sess.ID, sess.Root)
		for _, w := range sess.Worktrees {
			fmt.Printf("  - %s (%s)\n", w.Name, w.Path)
		}
	}
	return nil
}
