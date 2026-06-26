package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/JinmuGo/eme/internal/errors"
	"github.com/JinmuGo/eme/internal/gh"
	"github.com/JinmuGo/eme/internal/git"
	"github.com/JinmuGo/eme/internal/session"
	"github.com/JinmuGo/eme/internal/tui"
)

var (
	cloneDirFlag string
	cloneDryRun  bool
)

var cloneCmd = &cobra.Command{
	Use:   "clone [owner/repo | url]",
	Short: "Clone a GitHub repo (via gh) into a managed eme project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		setAgentChoiceFromFlag(cmd)
		var spec string
		if len(args) == 1 {
			spec = args[0]
		} else {
			picked, cancelled, err := pickRepo()
			if err != nil {
				return err
			}
			if cancelled {
				return nil // dismissed picker: do nothing (never an implicit target)
			}
			spec = picked
		}
		return runClone(spec)
	},
}

func init() {
	cloneCmd.Flags().StringVar(&cloneDirFlag, "dir", "", "directory to clone into (overrides [clone] dir)")
	cloneCmd.Flags().BoolVar(&cloneDryRun, "dry-run", false, "print planned actions without executing")
	cloneCmd.Flags().BoolVar(&noSwitchFlag, "no-switch", false, "do not switch the tmux client after creating")
	cloneCmd.Flags().StringVar(&newAgentFlag, "agent", "", `agent command to launch non-interactively ("none" for a bare shell); omit for the interactive picker`)
}

// runClone resolves the destination, guards it, clones, and registers the project.
func runClone(spec string) error {
	name := repoNameFromSpec(spec)
	if name == "" {
		return errors.New(errors.CodeInvalidFolder,
			"Could not derive a repo name from the spec.",
			fmt.Sprintf("%q has no repository segment.", spec),
			"Use OWNER/REPO, a GitHub URL, or a bare repo name.")
	}
	dir, err := resolveCloneDir()
	if err != nil {
		return err
	}
	dest := filepath.Join(dir, name)

	if cloneDryRun {
		fmt.Printf("[dry-run] would clone %s into %s (nested-bare)\n", spec, dest)
		return nil
	}

	// Destination guard: switch if already a managed project; never clobber.
	if info, statErr := os.Stat(dest); statErr == nil && info.IsDir() {
		s, lerr := loadState()
		if lerr != nil {
			return lerr
		}
		if sess := s.SessionByID(session.ID(dest)); sess != nil {
			return switchToSession(sess)
		}
		if empty, eerr := dirIsEffectivelyEmpty(dest); eerr == nil && !empty {
			return errors.New(errors.CodeInvalidFolder,
				fmt.Sprintf("%s already exists and is not empty.", dest),
				"eme will not clone over an existing directory.",
				"Remove it, choose another --dir, or open it with `eme new`.")
		}
	}

	if !gh.Available() {
		return errGhNotFound()
	}
	if !gh.Authed(context.Background()) {
		return errGhNotAuthed()
	}

	branch, err := cloneBareLayout(dest, spec)
	if err != nil {
		return err
	}
	fmt.Printf("Cloned %s (%s)\n", spec, branch)
	return registerNestedBareProject(dest)
}

// repoNameFromSpec returns the repository basename from an OWNER/REPO spec, a
// GitHub URL, or a bare name, stripping a trailing ".git".
func repoNameFromSpec(spec string) string {
	s := strings.TrimSpace(spec)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")
	// Both https://host/OWNER/REPO and git@host:OWNER/REPO end in a "/REPO" or
	// ":REPO" segment; take the last path/scp segment.
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// resolveCloneDir resolves where to place clones: --dir flag, then EME_CLONE_DIR,
// then [clone] dir, then the first existing standard code root, then ~/src. It is
// pure (no directory creation) so callers like scanFolders can reuse it safely.
func resolveCloneDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if cloneDirFlag != "" {
		return expandTilde(cloneDirFlag, home), nil
	}
	if v := os.Getenv("EME_CLONE_DIR"); v != "" {
		return expandTilde(v, home), nil
	}
	if cfg != nil && cfg.Clone.Dir != "" {
		return expandTilde(cfg.Clone.Dir, home), nil
	}
	for _, name := range []string{"Projects", "code", "src", "workspace", "dev", "Development"} {
		cand := filepath.Join(home, name)
		if info, serr := os.Stat(cand); serr == nil && info.IsDir() {
			return cand, nil
		}
	}
	return filepath.Join(home, "src"), nil
}

// pickRepo lists the user's GitHub repos and runs the interactive picker.
func pickRepo() (spec string, cancelled bool, err error) {
	if !gh.Available() {
		return "", false, errGhNotFound()
	}
	if !gh.Authed(context.Background()) {
		return "", false, errGhNotAuthed()
	}
	repos, err := gh.RepoList(context.Background(), 200)
	if err != nil {
		return "", false, err
	}
	items := make([]tui.RepoItem, len(repos))
	for i, r := range repos {
		items[i] = tui.RepoItem{NameWithOwner: r.NameWithOwner, Description: r.Description, Private: r.IsPrivate}
	}
	picker := tui.NewRepoPicker(items)
	if _, err := tea.NewProgram(picker, tea.WithAltScreen()).Run(); err != nil {
		return "", false, fmt.Errorf("picker: %w", err)
	}
	if picker.Cancelled() {
		return "", true, nil
	}
	return picker.Selected().NameWithOwner, false, nil
}

// cloneBareLayout builds eme's nested-bare layout from a remote clone.
func cloneBareLayout(dest, spec string) (string, error) {
	return cloneBareLayoutWith(context.Background(), dest, func(ctx context.Context) error {
		return gh.CloneBare(ctx, spec, filepath.Join(dest, ".bare"))
	})
}

// cloneBareLayoutWith is the testable core: clone (via cloneFn) into <dest>/.bare,
// fix the refspec, fetch, read the default branch, add the main worktree, and set
// upstream. On any failure after creation it removes what it made. Returns the
// checked-out default branch.
func cloneBareLayoutWith(ctx context.Context, dest string, cloneFn func(context.Context) error) (string, error) {
	bare := filepath.Join(dest, ".bare")
	mainWt := filepath.Join(dest, "main")

	createdDest := false
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		createdDest = true
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", fmt.Errorf("create destination: %w", err)
	}
	cleanup := func() {
		_ = git.WorktreeRemove(mainWt, true)
		_ = os.RemoveAll(bare)
		if createdDest {
			_ = os.RemoveAll(dest)
		}
	}

	if err := cloneFn(ctx); err != nil {
		cleanup()
		return "", err
	}
	if err := git.SetFetchRefspec(bare); err != nil {
		cleanup()
		return "", errors.Wrap(errors.CodeCommandFailed,
			"Failed to configure the clone's fetch refspec.",
			"git config remote.origin.fetch failed.",
			"Run with --verbose to see git output.", err)
	}
	if err := git.Fetch(bare); err != nil {
		cleanup()
		return "", errors.Wrap(errors.CodeCommandFailed,
			"Failed to fetch from origin.",
			"git fetch origin failed.",
			"Check your network and gh auth, then retry.", err)
	}
	branch, err := git.DefaultBranch(bare)
	if err != nil || branch == "" || branch == "HEAD" {
		branch = "main"
	}
	if err := git.WorktreeAddAt(bare, mainWt, branch, false); err != nil {
		cleanup()
		return "", errors.Wrap(errors.CodeCommandFailed,
			"Failed to create the main worktree.",
			"git worktree add failed.",
			"Run with --verbose to see git output.", err)
	}
	_ = git.SetUpstream(mainWt, branch, "origin/"+branch) // best-effort
	return branch, nil
}

func errGhNotFound() error {
	return errors.New(errors.CodeGhNotFound,
		"The GitHub CLI (gh) is not installed.",
		"eme clone uses gh to list and clone your repositories.",
		"Install gh from https://cli.github.com and run `gh auth login`.")
}

func errGhNotAuthed() error {
	return errors.New(errors.CodeGhNotAuthed,
		"The GitHub CLI (gh) is not authenticated.",
		"gh has no logged-in account.",
		"Run `gh auth login`, then try again.")
}
