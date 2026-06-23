package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestInput_BoxIsBordered verifies the prompt renders inside the rounded dialog frame, so
// the dashboard can overlay it as a modal (matching the agent picker's chrome).
func TestInput_BoxIsBordered(t *testing.T) {
	m := NewInput("Worktree name")
	box := m.Box()
	if !strings.Contains(box, "Worktree name") {
		t.Fatalf("box missing prompt:\n%s", box)
	}
	if !strings.ContainsAny(box, "╭╮╰╯") {
		t.Fatalf("input box is not bordered:\n%s", box)
	}
}

// TestInput_ViewCentersWithinWindow verifies the standalone input centers its dialog once
// it learns the terminal size, so it reads as a modal in the middle, not pinned top-left.
func TestInput_ViewCentersWithinWindow(t *testing.T) {
	m := NewInput("Worktree name")
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = model.(*InputModel)
	lines := strings.Split(m.View(), "\n")
	if len(lines) < 24 {
		t.Fatalf("centered view should fill the height (24 lines), got %d", len(lines))
	}
	if strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("expected blank top padding for vertical centering, got %q", lines[0])
	}
}
