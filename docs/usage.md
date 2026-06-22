# eme Usage Guide

## Overview

`eme` maps a project folder to a tmux session and each git worktree to a tmux window. AI agents run inside those windows.

## Install

```bash
go install github.com/jinmu/eme@latest
```

## Bind tmux

Add to `~/.tmux.conf`:

```tmux
bind-key a display-popup -E -w 80% -h 80% eme
```

Reload with `tmux source-file ~/.tmux.conf`.

## Your first project

1. Press `<prefix> a` to open the dashboard.
2. Press `n` and pick a folder.
3. `eme` creates `<folder>/main`, a tmux session, and a window.
4. Press `c` and type a worktree name.
5. Press `a` to launch your AI agent (or `A` to pick which one).

## CLI commands

```text
eme                # dashboard
eme new [folder]   # create project + main worktree
eme new --worktree <session> [name]  # create worktree
eme switch <session> [worktree]      # switch window
eme kill <session> [worktree]        # remove (needs --force)
eme agent <session> [worktree]       # toggle agent
eme agent <session> [worktree] --pick # choose the worktree's agent
eme doctor         # verify environment
eme --version      # print version
```

## Configuration

`~/.config/eme/config.toml`:

```toml
[agent]
command = "opencode"

[[agents]]
name = "claude-resume"
command = "claude --resume"
```

You can override the agent per folder or per worktree from the dashboard.

## Dashboard keys

The tree uses vim/nvim-style motions — sessions fold like a file tree.

| Key | Action |
|-----|--------|
| `↓` / `j`, `↑` / `k` | Move down / up (over session headers and worktrees) |
| `→` / `l` | Expand a folded session, step into a session, or open a worktree |
| `←` / `h` | Fold a session (from a worktree, fold its parent and jump to the header) |
| `Enter` / `o` | On a worktree: switch to it · On a session header: toggle fold |
| `p` | Peek the selected worktree's last pane lines (read-only) |
| `n` | New project |
| `c` | Create worktree in the session under the cursor |
| `d` | Kill the worktree, or the whole project on a `main`/header row |
| `a` | Toggle agent in the selected worktree |
| `A` | Pick the selected worktree's agent from the catalog |
| `x` | Reset a crashed/exited worktree's pane back to idle |
| `?` | Toggle help |
| `q` / `Esc` | Quit |

## Creating a worktree (`c`)

Type a name and eme does the right thing:

- **New name** → creates a new branch and worktree.
- **An existing branch** → checks that branch out into the new worktree (works for a
  local branch, or a remote branch eme tracks for you).
- **A branch already checked out** in a worktree eme manages → switches you there
  instead of erroring (a branch can live in only one worktree).
- **A name that collides as a path** (e.g. `feat` when `feat/x` and `feat/y` exist) →
  eme refuses and lists those `feat/*` branches; type one to check it out.

## Precise agent status (hooks)

By default eme infers status from the pane's foreground process, which can tell `idle`
from "something running" but not `working` from `waiting-for-input`. To make eme match
the agent exactly, let the agent push its real state:

```bash
eme hooks install      # Claude Code: wire status hooks into ~/.claude/settings.json
eme hooks uninstall    # remove them
```

This is **opt-in** and **merge-safe** — it preserves every other setting and any hooks
you already have, backs up your settings to `settings.json.eme-bak`, and is idempotent.
Restart the agent (or start a new one) for the hooks to take effect. Under the hood the
hook stamps a tmux pane option (`@eme_state`) that eme reads in its normal status poll;
agents without it installed keep working with the foreground heuristic.

Currently only Claude Code exposes the lifecycle hooks eme needs. One known gap: Claude's
blocking choice menus (AskUserQuestion) don't fire the notification hook, so that
particular waiting state isn't surfaced yet.

## Worktree layout

For a **new, empty** folder, `eme` creates a nested bare repository:

```text
<folder>/
  .bare/       # bare git repo
  main/        # main worktree
  feature/     # additional worktree
```

If you point `eme new` at a folder that already contains a git repo, it is
adopted in place (the clone is the `main` worktree; new worktrees go to a sibling
`<repo>.worktrees/`).

### Plain (non-git) folders

If you pick a folder that already has **content but is not a git repo** — for
example a multi-repo parent directory you want to run a top-level agent in — `eme`
adopts it as a **plain** project: it runs the agent in the folder directly and
creates no `.bare`/`main/` scaffolding. A plain project is a single window at the
folder root. Because there is no git, worktree creation (`c`) is unavailable — run
`git init` in the folder (and re-add it) if you want worktree-per-agent.

Run `eme doctor <folder>` to see which action a folder would take.

## Troubleshooting

- **tmux server is not running**: start one with `tmux new-session -d`.
- **Agent not found**: install the agent or set `agent.command`.
- **Session name ambiguous**: use the full session id shown in `eme`.
