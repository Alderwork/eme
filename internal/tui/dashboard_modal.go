package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// overlayModal is a dialog the dashboard embeds and draws centered over the live tree. Box
// renders just the bordered box (overlayCenter places it); the dashboard drives Update and
// detects completion through each model's own accessors. InputModel, AgentPickerModel, and
// FolderPickerModel all satisfy it.
type overlayModal interface {
	tea.Model
	Box() string
}

// flowKind is which multi-step action a modal sequence is carrying out.
type flowKind int

const (
	flowWorktree   flowKind = iota // c: name → agent → `new --worktree`
	flowAgentOnly                  // A: agent → `agent --set`
	flowNewProject                 // n: folder → agent → `new <folder>`
	flowClone                      // g: repo → agent → `clone <repo>`
)

// modalFlow is the context a modal sequence accumulates before it fires its background
// command: the session it targets and the name/folder gathered from earlier steps.
type modalFlow struct {
	kind    flowKind
	sessKey string // flowWorktree, flowAgentOnly: the session
	wtName  string // flowAgentOnly: the worktree; flowWorktree: filled after name entry
	folder  string // flowNewProject: filled after folder pick
	repo    string // flowClone: filled after repo pick
}

// SetAgentPicker injects the factory that builds an agent picker for a worktree (the cmd
// layer owns the catalog + PATH detection). sessionID/worktreeName may be "" for a target
// that does not exist yet (a new worktree or project), in which case the picker just
// defaults to the first installed agent.
func (m *DashboardModel) SetAgentPicker(fn func(sessionID, worktreeName string) *AgentPickerModel) {
	m.makeAgentPicker = fn
}

// SetFolderPicker injects the factory that builds the project-folder picker (the cmd layer
// owns the folder scan).
func (m *DashboardModel) SetFolderPicker(fn func() *FolderPickerModel) {
	m.makeFolderPicker = fn
}

// SetRepoFetcher injects the function that loads the user's GitHub repos for the clone
// picker. The cmd layer owns the gh call and the gh.Repo→RepoItem mapping, so tui stays free
// of gh. nil leaves the `g` action falling back to a child process.
func (m *DashboardModel) SetRepoFetcher(fn func() ([]RepoItem, error)) {
	m.fetchRepos = fn
}

// repoActionsWired reports whether the clone flow can run in-dashboard: it needs the agent
// picker (post-clone agent choice) and the repo fetcher.
func (m *DashboardModel) repoActionsWired() bool {
	return m.makeAgentPicker != nil && m.fetchRepos != nil
}

// modalsWired reports whether both picker factories are installed. When they are not (tests
// that exercise only the tree), the interactive actions fall back to child processes.
func (m *DashboardModel) modalsWired() bool {
	return m.makeAgentPicker != nil && m.makeFolderPicker != nil
}

// openModal makes mod the active dialog for flow, sizes it to the current terminal, and
// returns its initial command (e.g. the input cursor blink).
func (m *DashboardModel) openModal(mod overlayModal, flow *modalFlow) tea.Cmd {
	m.closePreview()
	m.notice = ""
	m.modal = mod
	m.flow = flow
	return m.sizeAndInit()
}

// sizeAndInit hands the active modal the current window size (so its box is laid out before
// the first paint) and returns its Init command.
func (m *DashboardModel) sizeAndInit() tea.Cmd {
	if m.width > 0 && m.height > 0 {
		updated, _ := m.modal.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		if om, ok := updated.(overlayModal); ok {
			m.modal = om
		}
	}
	return m.modal.Init()
}

// updateWithModal handles messages while a dialog is open: the dialog owns the keyboard,
// cursor blink, and resize; the auto-refresh tick merely reschedules so the frozen tree
// underneath does not churn; a finished child (which only fires after the modal closed)
// still refreshes defensively.
func (m *DashboardModel) updateWithModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, m.tick()
	case actionFinishedMsg:
		m.refresh(msg.err, msg.output)
		return m, nil
	case reposLoadedMsg:
		return m.onReposLoaded(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.updateModal(msg)
	}
	return m, m.updateModal(msg)
}

// closeModal tears down the active dialog and its flow.
func (m *DashboardModel) closeModal() {
	m.modal = nil
	m.flow = nil
}

// onReposLoaded swaps the loading modal for the repo picker when the fetch returns, or closes
// it with a notice on error. A result that arrives after the user cancelled (the loading modal
// is gone) is dropped.
func (m *DashboardModel) onReposLoaded(msg reposLoadedMsg) (tea.Model, tea.Cmd) {
	if _, ok := m.modal.(*LoadingModal); !ok {
		return m, nil // user cancelled before the load returned
	}
	if msg.err != nil {
		m.closeModal()
		m.notice = firstMeaningfulLine(msg.err.Error())
		if m.notice == "" {
			m.notice = "could not load GitHub repos"
		}
		return m, nil
	}
	m.modal = NewRepoPicker(msg.repos)
	return m, m.sizeAndInit()
}

// updateModal forwards msg to the active dialog, then advances the flow when the dialog
// reports completion. It NEVER returns the dialog's own tea.Quit (which would exit the whole
// dashboard): on completion it swallows that and returns the flow's next command instead.
func (m *DashboardModel) updateModal(msg tea.Msg) tea.Cmd {
	updated, cmd := m.modal.Update(msg)
	if om, ok := updated.(overlayModal); ok {
		m.modal = om
	}
	switch mod := m.modal.(type) {
	case *InputModel:
		if mod.Cancelled() {
			m.closeModal()
			return nil
		}
		if mod.Submitted() {
			return m.advanceFromName(mod.Value())
		}
	case *FolderPickerModel:
		if mod.Cancelled() {
			m.closeModal()
			return nil
		}
		if f := mod.Selected(); f != "" {
			return m.advanceFromFolder(f)
		}
	case *AgentPickerModel:
		if mod.Cancelled() {
			return m.advanceFromAgentCancel()
		}
		if sel, ok := mod.Chosen(); ok {
			return m.advanceFromAgent(sel)
		}
	case *LoadingModal:
		if mod.Cancelled() {
			m.closeModal()
			return nil
		}
	case *RepoPickerModel:
		if mod.Cancelled() {
			m.closeModal()
			return nil
		}
		if sel := mod.Selected(); sel.NameWithOwner != "" {
			return m.advanceFromRepo(sel.NameWithOwner)
		}
	}
	return cmd // still interacting — keep the dialog's own cmd (cursor blink), not tea.Quit
}

// advanceFromName moves the worktree-create flow from name entry to the agent picker. An
// empty name aborts (mirrors the CLI prompt, which refuses an empty name).
func (m *DashboardModel) advanceFromName(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if name == "" {
		m.closeModal()
		return nil
	}
	m.flow.wtName = name
	// The worktree does not exist yet, so the picker has no worktree-specific default.
	m.modal = m.makeAgentPicker(m.flow.sessKey, "")
	return m.sizeAndInit()
}

// advanceFromFolder moves the new-project flow from folder selection to the agent picker.
func (m *DashboardModel) advanceFromFolder(folder string) tea.Cmd {
	m.flow.folder = folder
	m.modal = m.makeAgentPicker("", "") // brand-new project: no current agent
	return m.sizeAndInit()
}

// advanceFromRepo moves the clone flow from repo selection to the agent picker.
func (m *DashboardModel) advanceFromRepo(spec string) tea.Cmd {
	m.flow.repo = spec
	m.modal = m.makeAgentPicker("", "") // brand-new project: no current agent
	return m.sizeAndInit()
}

// advanceFromAgent fires the flow's background command with the chosen agent ("none" for the
// — none — row, which leaves a bare shell).
func (m *DashboardModel) advanceFromAgent(sel AgentItem) tea.Cmd {
	agent := "none"
	if !sel.None {
		agent = sel.Command
	}
	return m.runFlow(agent)
}

// advanceFromAgentCancel handles Esc on the agent picker. For a re-pick (A) it aborts with no
// change; for create/new-project it proceeds with no agent — the worktree/project is still
// created, matching the CLI flows where cancelling the picker leaves a bare shell.
func (m *DashboardModel) advanceFromAgentCancel() tea.Cmd {
	if m.flow.kind == flowAgentOnly {
		m.closeModal()
		return nil
	}
	return m.runFlow("none")
}

// runFlow closes the modal and dispatches the flow's `eme` child in the background (no
// terminal handoff, so the dashboard never clears). A create flow first arms a pending
// switch so the dashboard lands on the new window once the child returns.
func (m *DashboardModel) runFlow(agent string) tea.Cmd {
	args := m.flowArgs(agent)
	m.armCreateSwitch()
	m.closeModal()
	return m.runChildBackground(args...)
}

// createSwitch records what a completed create flow should switch to. The new
// session/worktree is identified against the post-create reload rather than guessed up
// front, so it survives the path-derived session id (new project) and the destination a
// clone resolves to.
type createSwitch struct {
	kind    flowKind
	sessKey string // flowWorktree: the parent session whose new worktree to land on
	wtName  string // flowWorktree: the created worktree's name
	// prevSessions is the set of session keys that existed before a flowNewProject/
	// flowClone create, so the one missing from it afterward is the new project.
	prevSessions map[string]bool
}

// armCreateSwitch records, for a create flow, what the dashboard should switch to once the
// background child succeeds. A worktree create knows its (session, name) up front; a new
// project / clone cannot (the session id is derived from a path the tui never sees), so it
// snapshots the current session keys and finds the new one after the reload. A re-pick
// (flowAgentOnly) changes nothing to land on, so it arms nothing.
func (m *DashboardModel) armCreateSwitch() {
	f := m.flow
	if f == nil {
		return
	}
	switch f.kind {
	case flowWorktree:
		m.pendingSwitch = &createSwitch{kind: f.kind, sessKey: f.sessKey, wtName: f.wtName}
	case flowNewProject, flowClone:
		m.pendingSwitch = &createSwitch{kind: f.kind, prevSessions: m.sessionKeySet()}
	}
}

// resolveCreateSwitch consumes a create flow's armed switch after its child returns. On
// success it locates the new worktree (flowWorktree) or new project (flowNewProject/
// flowClone) in the just-reloaded views and, when found, records it as the switch target
// and quits so the cmd layer execs `eme switch`. A failed child, an absent target (a
// silent no-op, or a name that resolved to an existing worktree), or no arm all leave the
// dashboard in place.
func (m *DashboardModel) resolveCreateSwitch(actionErr error) tea.Cmd {
	cs := m.pendingSwitch
	if cs == nil {
		return nil
	}
	m.pendingSwitch = nil
	if actionErr != nil {
		return nil // the create failed; refresh already surfaced why — stay put
	}
	sid, wt, ok := cs.target(m.views)
	if !ok {
		return nil
	}
	m.leaving = true
	m.leaveSession, m.leaveWorktree = sid, wt
	return tea.Quit
}

// target finds the session id + worktree name the completed create should switch to,
// against the post-create views. ok is false when the expected entry is absent, so the
// caller stays on the dashboard instead of switching to something that is not there.
func (cs *createSwitch) target(views []SessionView) (sessionID, worktree string, ok bool) {
	switch cs.kind {
	case flowWorktree:
		for si := range views {
			if sessionKey(views[si]) != cs.sessKey {
				continue
			}
			for i := range views[si].Worktrees {
				if w := views[si].Worktrees[i]; w.Name == cs.wtName {
					return w.SessionID, w.Name, true
				}
			}
		}
	case flowNewProject, flowClone:
		for si := range views {
			if cs.prevSessions[sessionKey(views[si])] {
				continue
			}
			if w := landingWorktree(views[si]); w != nil {
				return w.SessionID, w.Name, true
			}
		}
	}
	return "", "", false
}

// landingWorktree picks the worktree to land on in a freshly created project: its main
// worktree (every project layout — nested-bare, adopted, plain — registers one), falling
// back to the first worktree so a switch still happens if "main" is ever absent.
func landingWorktree(sv SessionView) *WorktreeView {
	for i := range sv.Worktrees {
		if sv.Worktrees[i].IsMain || sv.Worktrees[i].Name == "main" {
			return &sv.Worktrees[i]
		}
	}
	if len(sv.Worktrees) > 0 {
		return &sv.Worktrees[0]
	}
	return nil
}

// sessionKeySet snapshots the identity of every session currently in the view-model, so a
// new-project / clone flow can tell which session is new once its child returns.
func (m *DashboardModel) sessionKeySet() map[string]bool {
	keys := make(map[string]bool, len(m.views))
	for si := range m.views {
		keys[sessionKey(m.views[si])] = true
	}
	return keys
}

// flowArgs is the pure mapping from the active flow + chosen agent to the `eme` child argv —
// split out from runFlow so the EXACT argv is unit-testable without spawning a process.
func (m *DashboardModel) flowArgs(agent string) []string {
	f := m.flow
	if f == nil {
		return nil
	}
	switch f.kind {
	case flowWorktree:
		return worktreeCreateArgs(f.sessKey, f.wtName, agent)
	case flowAgentOnly:
		return agentSetArgs(f.sessKey, f.wtName, agent)
	case flowNewProject:
		return newProjectArgs(f.folder, agent)
	case flowClone:
		return cloneArgs(f.repo, agent)
	}
	return nil
}

// worktreeCreateArgs builds `eme new --worktree <sess> <name> --no-switch --agent <agent>`:
// the name positional skips the input prompt, --agent skips the picker, --no-switch keeps the
// dashboard's client in place.
func worktreeCreateArgs(sessKey, name, agent string) []string {
	return []string{"new", "--worktree", sessKey, name, "--no-switch", "--agent", agent}
}

// agentSetArgs builds `eme agent <sess> <wt> --set <agent>` — set + launch the chosen agent
// non-interactively.
func agentSetArgs(sessKey, wtName, agent string) []string {
	return []string{"agent", sessKey, wtName, "--set", agent}
}

// newProjectArgs builds `eme new <folder> --no-switch --agent <agent>` — create the project
// at the picked folder with the chosen agent, no picker, client unchanged.
func newProjectArgs(folder, agent string) []string {
	return []string{"new", folder, "--no-switch", "--agent", agent}
}

// cloneArgs builds `eme clone <spec> --no-switch --agent <agent>` — clone the repo with the
// chosen agent, no picker, the dashboard's client left in place.
func cloneArgs(spec, agent string) []string {
	return []string{"clone", spec, "--no-switch", "--agent", agent}
}
