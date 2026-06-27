package cmd

import (
	"strings"
	"testing"

	"github.com/alderwork/eme/internal/runner"
)

// TestRenderSegment locks the segment formatting: dark when nothing waits, a
// glyph-led ●N beacon when agents wait for your input. The segment is waiting-only
// — a crash is surfaced in the dashboard, never in the bar.
func TestRenderSegment(t *testing.T) {
	if got := renderSegment(0); got != "" {
		t.Errorf("nothing waiting → %q, want empty (dark cockpit)", got)
	}
	if got := renderSegment(3); !strings.Contains(got, "●3") {
		t.Errorf("3 waiting → %q, want to contain ●3", got)
	}
	if got := renderSegment(1); strings.Contains(got, "✗") {
		t.Errorf("waiting → %q, must not show a crash glyph (segment is waiting-only)", got)
	}
}

// TestRenderSegmentClosesColorSpan guards the tmux color enhancement: a colored
// segment must close its #[fg=...] span so it never bleeds into the rest of the bar.
func TestRenderSegmentClosesColorSpan(t *testing.T) {
	got := renderSegment(1)
	if strings.Contains(got, "#[fg=") && !strings.HasSuffix(got, "#[fg=default]") {
		t.Errorf("segment %q opens a color span without closing it", got)
	}
}

// TestStatusSegment_EmptyWhenUnavailable locks the status-bar contract: a read
// failure degrades to an empty segment, never an error printed into the user's bar.
func TestStatusSegment_EmptyWhenUnavailable(t *testing.T) {
	tempState(t)
	prev := runner.Default
	runner.Default = runner.NewMock() // unstubbed → snapshot read fails
	t.Cleanup(func() { runner.Default = prev })

	if got := statusSegment(); got != "" {
		t.Errorf("statusSegment with no tmux = %q, want empty (degraded, no error)", got)
	}
}
