package tui

import (
	"fmt"
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

// TestRenderSuggestions_Cap verifies that a long candidate list is truncated
// to suggestLineCap entries and a "(+N)" suffix is appended.
func TestRenderSuggestions_Cap(t *testing.T) {
	// Build a list longer than suggestLineCap.
	cands := make([]string, suggestLineCap+10)
	for i := range cands {
		cands[i] = fmt.Sprintf("item%03d", i)
	}
	out := renderSuggestions(cands)
	if !strings.Contains(out, fmt.Sprintf("… (+%d)", 10)) {
		t.Errorf("renderSuggestions cap: missing overflow suffix; got: %q", out)
	}
	// Only the first suggestLineCap items should be visible.
	for i := 0; i < suggestLineCap; i++ {
		if !strings.Contains(out, fmt.Sprintf("item%03d", i)) {
			t.Errorf("renderSuggestions: missing item%03d in %q", i, out)
		}
	}
	// The (suggestLineCap+1)th item must NOT appear in the visible portion.
	if strings.Contains(out, fmt.Sprintf("item%03d", suggestLineCap)) {
		t.Errorf("renderSuggestions: item%03d should not appear before overflow suffix", suggestLineCap)
	}
}

// TestRenderSuggestions_NoCap verifies that a short list is rendered without
// a truncation suffix.
func TestRenderSuggestions_NoCap(t *testing.T) {
	cands := []string{"alpha", "beta", "gamma"}
	out := renderSuggestions(cands)
	if strings.Contains(out, "…") {
		t.Errorf("renderSuggestions short list: unexpected overflow suffix in %q", out)
	}
	for _, c := range cands {
		if !strings.Contains(out, c) {
			t.Errorf("renderSuggestions short list: missing %q in %q", c, out)
		}
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

func TestRenderHostListShowsBadges(t *testing.T) {
	st := map[string]hostStatus{
		"vm-a": {bgState: "Running", ready: 3, total: 3, reachable: true},
		"vm-b": {bgState: "Stopped", ready: 0, total: 3, reachable: false},
	}
	out := renderHostList([]string{"vm-a", "vm-b"}, st, 0)
	if !strings.Contains(out, "●") || !strings.Contains(out, "○") {
		t.Fatalf("expected reachable/unreachable badges:\n%s", out)
	}
}
