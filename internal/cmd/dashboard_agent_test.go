package cmd

import (
	"reflect"
	"testing"
)

func TestDashboardAgentArgs_ToggleAndPick(t *testing.T) {
	m := dashboardWith("myapp", "feat", false) // helper from dashboard_switch_test.go

	got, ok := m.AgentArgs(false)
	if !ok || !reflect.DeepEqual(got, []string{"agent", "myapp", "feat"}) {
		t.Errorf("AgentArgs(false) = %v (ok=%v), want [agent myapp feat]", got, ok)
	}

	gotPick, ok := m.AgentArgs(true)
	if !ok || !reflect.DeepEqual(gotPick, []string{"agent", "myapp", "feat", "--pick"}) {
		t.Errorf("AgentArgs(true) = %v (ok=%v), want [agent myapp feat --pick]", gotPick, ok)
	}
}
