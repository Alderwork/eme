package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleItems() []AgentItem {
	return []AgentItem{
		{Name: "claude", Command: "claude", Installed: true},
		{Name: "codex", Command: "codex", Installed: false},
		{Name: "opencode", Command: "opencode", Installed: true},
		{Name: "none", None: true, Installed: true},
	}
}

func key(m *AgentPickerModel, t tea.KeyType) *AgentPickerModel {
	model, _ := m.Update(tea.KeyMsg{Type: t})
	return model.(*AgentPickerModel)
}

// TestAgentPicker_BoxBeforeWindowSize guards that the dashboard can render the picker's box
// the instant it opens it — before any WindowSizeMsg (sizeAndInit skips sizing when the
// terminal size is still 0). content() must not depend on width/height.
func TestAgentPicker_BoxBeforeWindowSize(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude") // no WindowSizeMsg yet
	box := m.Box()
	if !strings.Contains(box, "Pick an agent") {
		t.Errorf("Box() before sizing should still render the title; got %q", box)
	}
	if !strings.Contains(box, "╭") {
		t.Error("Box() should render the rounded border even before sizing")
	}
}

func TestAgentPicker_EnterSelectsInstalled(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude") // cursor starts on claude
	m = key(m, tea.KeyEnter)
	sel, ok := m.Chosen()
	if !ok || sel.Name != "claude" {
		t.Fatalf("Chosen() = %+v, %v; want claude, true", sel, ok)
	}
	if m.Cancelled() {
		t.Errorf("Cancelled() = true, want false")
	}
}

func TestAgentPicker_SkipsUninstalledOnNavigation(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude") // index 0 (claude)
	m = key(m, tea.KeyDown)                      // must skip codex (index 1) → opencode (index 2)
	m = key(m, tea.KeyEnter)
	sel, ok := m.Chosen()
	if !ok || sel.Name != "opencode" {
		t.Fatalf("after Down+Enter Chosen() = %+v, %v; want opencode, true", sel, ok)
	}
}

func TestAgentPicker_SelectsNone(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude")
	m = key(m, tea.KeyDown) // opencode
	m = key(m, tea.KeyDown) // none
	m = key(m, tea.KeyEnter)
	sel, ok := m.Chosen()
	if !ok || !sel.None {
		t.Fatalf("Chosen() = %+v, %v; want none row, true", sel, ok)
	}
}

func TestAgentPicker_EscCancels(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude")
	m = key(m, tea.KeyEsc)
	if _, ok := m.Chosen(); ok {
		t.Errorf("Chosen() ok = true after Esc, want false")
	}
	if !m.Cancelled() {
		t.Errorf("Cancelled() = false, want true")
	}
}

func TestNewAgentPicker_DefaultHighlightSkipsUninstalled(t *testing.T) {
	// default points at an uninstalled agent → cursor falls to first installed.
	m := NewAgentPicker(sampleItems(), "codex")
	if got := m.items[m.cursor].Name; got != "claude" {
		t.Errorf("initial cursor on %q, want first installed 'claude'", got)
	}
}

// TestAgentPicker_JKNavigates: j/k move the cursor like ↓/↑ (skipping uninstalled
// rows), matching the dashboard's vim-style navigation.
func TestAgentPicker_JKNavigates(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude") // cursor on claude (index 0)
	model, _ := m.Update(runeKey('j'))           // down, skipping uninstalled codex → opencode
	m = model.(*AgentPickerModel)
	if got := m.items[m.cursor].Name; got != "opencode" {
		t.Fatalf("after j, cursor on %q, want opencode", got)
	}
	model, _ = m.Update(runeKey('k')) // up → back to claude
	m = model.(*AgentPickerModel)
	if got := m.items[m.cursor].Name; got != "claude" {
		t.Fatalf("after k, cursor on %q, want claude", got)
	}
}

func TestAgentPicker_UpAtTopIsNoop(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude") // cursor on claude (index 0)
	m = key(m, tea.KeyUp)                        // already at top → no-op
	m = key(m, tea.KeyEnter)
	sel, ok := m.Chosen()
	if !ok || sel.Name != "claude" {
		t.Fatalf("Up at top then Enter = %+v, %v; want claude, true", sel, ok)
	}
}

// TestAgentPicker_ViewIsBorderedBox verifies the picker renders inside a bordered
// dialog frame (the modal), not as a bare top-left list. The rounded-border corners
// mirror the dashboard's panel so the modal reads as the same chrome.
func TestAgentPicker_ViewIsBorderedBox(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude")
	view := m.View()
	if !strings.Contains(view, "Pick an agent") {
		t.Fatalf("view missing title:\n%s", view)
	}
	if !strings.ContainsAny(view, "╭╮╰╯") {
		t.Fatalf("view is not wrapped in a bordered box:\n%s", view)
	}
}

// TestAgentPicker_ViewCentersWithinWindow verifies that once the picker learns the
// terminal size (WindowSizeMsg) it centers the dialog — blank padding above the box
// and filling the full height — so it reads as a modal in the middle, not pinned to
// the top-left corner.
func TestAgentPicker_ViewCentersWithinWindow(t *testing.T) {
	m := NewAgentPicker(sampleItems(), "claude")
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = model.(*AgentPickerModel)
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) < 24 {
		t.Fatalf("centered view should fill the window height (24 lines), got %d:\n%s", len(lines), view)
	}
	if strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("expected blank top padding for vertical centering, got %q", lines[0])
	}
	if !strings.Contains(view, "Pick an agent") {
		t.Fatalf("centered view lost its content:\n%s", view)
	}
}
