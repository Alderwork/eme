package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// reset closes any open SGR so a composited segment never bleeds its color/weight into the
// next one when lines from two independently styled renders are spliced together.
const reset = "\x1b[0m"

// overlayCenter composites fg centered over bg, preserving the parts of bg that fg does not
// cover — the basis for drawing a modal box on top of the live dashboard instead of clearing
// it. Both strings may carry ANSI styling; the splice is display-width aware so columns stay
// aligned regardless of color codes.
func overlayCenter(bg, fg string) string {
	bgW, bgH := dims(strings.Split(bg, "\n"))
	fgW, fgH := dims(strings.Split(fg, "\n"))
	return overlay(bg, fg, (bgW-fgW)/2, (bgH-fgH)/2)
}

// overlay composites fg over bg with fg's top-left at column col, row row (both clamped to
// >= 0). Rows of bg outside fg's vertical span pass through unchanged; within it, a bg line
// keeps its first col columns, fg's line replaces the next fgWidth columns, and bg resumes
// after — so the box punches a hole in the dashboard exactly its own size.
func overlay(bg, fg string, col, row int) string {
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	fgW, _ := dims(fgLines)

	for i, fl := range fgLines {
		r := row + i
		if r >= len(bgLines) {
			break // fg taller than bg; the modal always fits in practice, so drop the overflow
		}
		bgLine := bgLines[r]
		left := padTo(ansi.Truncate(bgLine, col, ""), col) // first col columns, padded if bg is short
		right := ansi.TruncateLeft(bgLine, col+fgW, "")     // bg resumes after the box
		bgLines[r] = left + reset + padTo(fl, fgW) + reset + right
	}
	return strings.Join(bgLines, "\n")
}

// dims returns the max display width and the line count of lines.
func dims(lines []string) (w, h int) {
	for _, l := range lines {
		if x := ansi.StringWidth(l); x > w {
			w = x
		}
	}
	return w, len(lines)
}

// padTo right-pads s with spaces to exactly w display columns (no-op when already >= w).
func padTo(s string, w int) string {
	if d := w - ansi.StringWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}
