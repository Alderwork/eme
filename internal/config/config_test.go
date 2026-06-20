package config

import "testing"

func TestCatalog_IncludesBuiltins(t *testing.T) {
	c := Default() // Agent.Command == "opencode"
	got := c.Catalog()
	names := map[string]string{}
	for _, a := range got {
		names[a.Name] = a.Command
	}
	for _, want := range []string{"claude", "codex", "gemini", "opencode"} {
		if names[want] != want {
			t.Errorf("catalog missing builtin %q (command=%q); got %v", want, names[want], names)
		}
	}
}

func TestCatalog_UserOverridesBuiltinCommandByName(t *testing.T) {
	c := Default()
	c.Agents = []AgentSpec{{Name: "claude", Command: "claude --resume"}}
	for _, a := range c.Catalog() {
		if a.Name == "claude" && a.Command != "claude --resume" {
			t.Errorf("claude command = %q, want override %q", a.Command, "claude --resume")
		}
	}
}

func TestCatalog_AppendsCustomAgent(t *testing.T) {
	c := Default()
	c.Agents = []AgentSpec{{Name: "aider", Command: "aider"}}
	found := false
	for _, a := range c.Catalog() {
		if a.Name == "aider" && a.Command == "aider" {
			found = true
		}
	}
	if !found {
		t.Errorf("custom agent 'aider' not in catalog: %v", c.Catalog())
	}
}

func TestCatalog_SurfacesCustomLegacyCommand(t *testing.T) {
	c := Default()
	c.Agent.Command = "my-agent --flag" // not a builtin
	found := false
	for _, a := range c.Catalog() {
		if a.Command == "my-agent --flag" {
			found = true
		}
	}
	if !found {
		t.Errorf("legacy agent.command not surfaced in catalog: %v", c.Catalog())
	}
}

func TestWorktreeDirFor_DefaultSibling(t *testing.T) {
	got, err := WorktreeDirFor("{repo}.worktrees", "/p/myapp")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "/p/myapp.worktrees" {
		t.Errorf("got %q", got)
	}
}

func TestWorktreeDirFor_RejectsAbsolute(t *testing.T) {
	if _, err := WorktreeDirFor("/abs/{repo}", "/p/myapp"); err == nil {
		t.Errorf("expected rejection of absolute template")
	}
}

func TestWorktreeDirFor_RejectsParentEscape(t *testing.T) {
	if _, err := WorktreeDirFor("../{repo}.wt", "/p/myapp"); err == nil {
		t.Errorf("expected rejection of parent-escaping template")
	}
}

func TestDefault_WorktreeTemplate(t *testing.T) {
	if Default().Worktree.DirTemplate != "{repo}.worktrees" {
		t.Errorf("default template = %q", Default().Worktree.DirTemplate)
	}
}
