package tui

import "testing"

// TestAgentStatusGlyphASCIIFallback: EME_ASCII opts a non-Unicode terminal into the
// DESIGN §6.4 ASCII status set, so the glyph channel (the colorblind/no-color backbone)
// still reads when the Unicode dots can't render.
func TestAgentStatusGlyphASCIIFallback(t *testing.T) {
	t.Setenv("EME_ASCII", "1")
	cases := map[AgentStatus]string{
		StatusWaiting: "*",
		StatusWorking: "o",
		StatusExited:  ".",
		StatusIdle:    "·",
		StatusCrashed: "x",
	}
	for s, want := range cases {
		if got := s.Glyph(); got != want {
			t.Errorf("EME_ASCII Glyph(%v) = %q, want %q", s, got, want)
		}
	}
}

func TestAgentStatusGlyphLabel(t *testing.T) {
	cases := []struct {
		s     AgentStatus
		glyph string
		label string
	}{
		{StatusWaiting, "●", "waiting"},
		{StatusWorking, "◐", "running"},
		{StatusExited, "○", "exited"},
		{StatusIdle, "·", "idle"},
		{StatusCrashed, "✗", "crashed"},
	}
	for _, c := range cases {
		if got := c.s.Glyph(); got != c.glyph {
			t.Errorf("Glyph(%v) = %q, want %q", c.s, got, c.glyph)
		}
		if got := c.s.Label(); got != c.label {
			t.Errorf("Label(%v) = %q, want %q", c.s, got, c.label)
		}
	}
}

func TestAgentStatusNeedsAttention(t *testing.T) {
	want := map[AgentStatus]bool{
		StatusWaiting: true,
		StatusCrashed: true,
		StatusExited:  false,
		StatusWorking: false,
		StatusIdle:    false,
	}
	for s, w := range want {
		if got := s.NeedsAttention(); got != w {
			t.Errorf("NeedsAttention(%v) = %v, want %v", s, got, w)
		}
	}
}
