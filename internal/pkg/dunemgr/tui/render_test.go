package tui

import (
	"strings"
	"testing"
)

func TestRenderHostListMarksSelectionAndStatus(t *testing.T) {
	hosts := []string{"vm-a", "vm-b"}
	st := map[string]hostStatus{
		"vm-a": {bgState: "RUNNING", ready: 2, total: 2, reachable: true},
		"vm-b": {bgState: "UNKNOWN", reachable: false, err: "probe error"},
	}
	out := renderHostList(hosts, st, 0)
	if !strings.Contains(out, "vm-a") || !strings.Contains(out, "RUNNING") {
		t.Errorf("host list missing vm-a/RUNNING:\n%s", out)
	}
	if !strings.Contains(out, "2/2") {
		t.Errorf("host list missing pod count:\n%s", out)
	}
	// selected row (index 0) is marked with the cursor '>'
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[0], ">") {
		t.Errorf("selected row not marked: %q", lines[0])
	}
}

func TestRenderDetailShowsErrorWhenPresent(t *testing.T) {
	d := renderDetail("vm-b", hostStatus{bgState: "UNKNOWN", reachable: false, err: "probe error"})
	if !strings.Contains(d, "vm-b") || !strings.Contains(d, "probe error") {
		t.Errorf("detail missing host/error:\n%s", d)
	}
}

func TestRenderEventLogShowsRecentNewestLast(t *testing.T) {
	out := renderEvents([]string{"vm-a: restart → ok", "vm-b: stop → ok"}, 10)
	if !strings.Contains(out, "vm-b: stop → ok") {
		t.Errorf("event log missing newest entry:\n%s", out)
	}
}

func TestRenderViewIncludesAllPanes(t *testing.T) {
	m := newModel(nil, nil)
	m.hosts = []string{"vm-a"}
	m.statuses = map[string]hostStatus{"vm-a": {bgState: "RUNNING", ready: 2, total: 2, reachable: true}}
	m.events = []string{"vm-a: restart → ok"}
	m.width, m.height = 80, 24
	v := m.View()
	for _, want := range []string{"vm-a", "RUNNING", "restart → ok"} {
		if !strings.Contains(v, want) {
			t.Errorf("View missing %q:\n%s", want, v)
		}
	}
}
