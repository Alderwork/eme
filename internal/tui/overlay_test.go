package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TestOverlayCenter_PlainSplicesAndPreservesWidth checks the core compositing on un-styled
// text: fg lands centered, the covered columns are exactly fg, the surrounding bg survives,
// and every row keeps the background width (alignment intact).
func TestOverlayCenter_PlainSplicesAndPreservesWidth(t *testing.T) {
	bg := strings.Join([]string{
		"ABCDEFGHIJ",
		"ABCDEFGHIJ",
		"ABCDEFGHIJ",
		"ABCDEFGHIJ",
		"ABCDEFGHIJ",
	}, "\n")
	fg := strings.Join([]string{"##", "##", "##"}, "\n") // 2 wide, 3 tall → col=4, row=1

	got := overlayCenter(bg, fg)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("row count changed: got %d, want 5", len(lines))
	}
	want := []string{
		"ABCDEFGHIJ", // row 0 untouched
		"ABCD##GHIJ", // rows 1-3 spliced at col 4
		"ABCD##GHIJ",
		"ABCD##GHIJ",
		"ABCDEFGHIJ", // row 4 untouched
	}
	for i, l := range lines {
		if stripped := ansi.Strip(l); stripped != want[i] {
			t.Errorf("row %d = %q, want %q", i, stripped, want[i])
		}
		if w := ansi.StringWidth(l); w != 10 {
			t.Errorf("row %d width = %d, want 10 (alignment broken)", i, w)
		}
	}
}

// TestOverlay_LeavesUncoveredRowsByteIdentical guards that rows outside the box are passed
// through verbatim — styling and all — so the dashboard behind the modal is undisturbed.
func TestOverlay_LeavesUncoveredRowsByteIdentical(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render("dashboard row")
	bg := strings.Join([]string{styled, "plain", styled}, "\n")
	got := overlay(bg, "X", 0, 1) // cover only the middle row
	lines := strings.Split(got, "\n")
	if lines[0] != styled {
		t.Errorf("row 0 was modified: %q != %q", lines[0], styled)
	}
	if lines[2] != styled {
		t.Errorf("row 2 was modified: %q != %q", lines[2], styled)
	}
}

// TestOverlayCenter_StyledBoxKeepsAlignment composites a real bordered lipgloss box over a
// styled background and asserts every row still occupies the full background width — the
// property that keeps the panel border from tearing when a modal is drawn over it.
func TestOverlayCenter_StyledBoxKeepsAlignment(t *testing.T) {
	row := lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Render(strings.Repeat("·", 40))
	var bgRows []string
	for range 12 {
		bgRows = append(bgRows, row)
	}
	bg := strings.Join(bgRows, "\n")
	box := dialogStyle.Render("Pick an agent\n\n> claude\n  codex")

	got := overlayCenter(bg, box)
	for i, l := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(l); w != 40 {
			t.Fatalf("row %d width = %d, want 40 (box tore the background)", i, w)
		}
	}
	if !strings.Contains(ansi.Strip(got), "Pick an agent") {
		t.Errorf("composited output lost the box content")
	}
}
