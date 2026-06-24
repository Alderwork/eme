package cmd

import (
	"testing"
	"time"

	"github.com/jinmu/eme/internal/tui"
)

func TestAnyWorking(t *testing.T) {
	if anyWorking(nil) {
		t.Fatal("nil → false")
	}
	if anyWorking([]tui.AgentStatus{tui.StatusIdle, tui.StatusExited}) {
		t.Fatal("no working → false")
	}
	if !anyWorking([]tui.AgentStatus{tui.StatusIdle, tui.StatusWorking}) {
		t.Fatal("one working → true")
	}
}

func TestShouldAssert(t *testing.T) {
	grace := 60 * time.Second
	if !shouldAssert(true, 0, grace) {
		t.Fatal("working → assert")
	}
	if !shouldAssert(false, 30*time.Second, grace) {
		t.Fatal("idle within grace → assert")
	}
	if shouldAssert(false, 90*time.Second, grace) {
		t.Fatal("idle past grace → release")
	}
	if shouldAssert(false, 10*time.Second, 0) {
		t.Fatal("zero grace, idle → release")
	}
}

func TestNormalizeMode(t *testing.T) {
	for _, in := range []string{"off", "manual", "auto"} {
		if got, err := normalizeMode(in); err != nil || got != in {
			t.Fatalf("normalizeMode(%q) = %q,%v", in, got, err)
		}
	}
	if got, err := normalizeMode("OFF"); err != nil || got != "off" {
		t.Fatalf("normalizeMode(OFF) = %q,%v want off", got, err)
	}
	if _, err := normalizeMode("nope"); err == nil {
		t.Fatal("invalid mode must error")
	}
}
