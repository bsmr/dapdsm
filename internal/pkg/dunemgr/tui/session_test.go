package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestCommandBarSession is an end-to-end, TTY-free integration test: it drives
// the model through a realistic operator session — poll-fed status panes,
// navigation, the ':' command bar with live suggestions, Tab-completion across
// argument positions, the ':help' built-in, and a live action frame — asserting
// on model state and the rendered View() at each step. It complements the
// per-feature unit tests by exercising the whole flow as the operator sees it.
func TestCommandBarSession(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a", "vm-b"}
	m.statuses = map[string]hostStatus{}

	apply := func(msg tea.Msg) { nm, _ := m.Update(msg); m = nm.(model) }
	keys := func(s string) { apply(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}) }
	// wantView asserts the rendered frame contains each substring, dumping the
	// whole frame on failure (so a regression shows the actual UI).
	wantView := func(step string, subs ...string) {
		t.Helper()
		v := m.View()
		for _, s := range subs {
			if !strings.Contains(v, s) {
				t.Errorf("[%s] View() missing %q\n----- frame -----\n%s", step, s, v)
			}
		}
	}

	// Step 1: window size + poll frames populate the status panes.
	apply(tea.WindowSizeMsg{Width: 78, Height: 18})
	apply(pollMsg{host: "vm-a", kind: pollBG, bgState: "RUNNING", ready: 2, total: 2})
	apply(pollMsg{host: "vm-a", kind: pollHealth, reachable: true})
	apply(pollMsg{host: "vm-b", kind: pollBG, bgState: "DEGRADED", ready: 1, total: 2})
	apply(pollMsg{host: "vm-b", kind: pollHealth, reachable: true})

	if m.selected != 0 {
		t.Fatalf("initial selection = %d, want 0", m.selected)
	}
	if got := m.statuses["vm-a"]; got.bgState != "RUNNING" || got.ready != 2 || !got.reachable {
		t.Fatalf("vm-a status not folded: %+v", got)
	}
	wantView("1 initial", "vm-a", "RUNNING", "2/2", "vm-b", "DEGRADED", "1/2", "reachable")

	// Step 2: arrow-down selects vm-b; the detail pane follows the selection.
	apply(tea.KeyMsg{Type: tea.KeyDown})
	if m.selected != 1 {
		t.Fatalf("after down: selection = %d, want 1", m.selected)
	}

	// Step 3: ':' enters command mode; typing shows a live suggestion line.
	keys(":")
	if m.mode != modeCmd {
		t.Fatalf("':' did not enter command mode (mode=%v)", m.mode)
	}
	keys("lifec")
	if m.input.Value() != "lifec" {
		t.Fatalf("input = %q, want \"lifec\"", m.input.Value())
	}
	wantView("3 suggest", ":lifec", "lifecycle") // prompt + live suggestion

	// Step 4: Tab completes the verb (unique → full value + trailing space).
	apply(tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != "lifecycle " {
		t.Fatalf("after Tab on 'lifec': input = %q, want \"lifecycle \"", m.input.Value())
	}

	// Step 5: Tab across positions — host, then sub-verb.
	keys("vm-a re")
	apply(tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != "lifecycle vm-a restart " {
		t.Fatalf("after Tab on 'vm-a re': input = %q, want \"lifecycle vm-a restart \"", m.input.Value())
	}

	// Step 6: ':help' (a built-in, not a dispatched verb) renders into the
	// output pane and returns to navigation mode.
	apply(tea.KeyMsg{Type: tea.KeyEsc})
	keys(":")
	keys("help")
	apply(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNav {
		t.Fatalf("after ':help' Enter: mode = %v, want modeNav", m.mode)
	}
	if m.input.Value() != "" {
		t.Errorf("input not cleared after Enter: %q", m.input.Value())
	}
	wantView("6 help", "── result ──", "lifecycle <host>", "backup", "shutdown", "help [verb]")

	// Step 7: a live action frame appends to the event log and the detail pane.
	apply(pollMsg{host: "vm-b", kind: pollAction, action: "restart", result: "ok"})
	if len(m.events) == 0 {
		t.Fatal("action frame did not append to the event log")
	}
	if got := m.statuses["vm-b"].lastAction; !strings.Contains(got, "restart") {
		t.Errorf("vm-b lastAction = %q, want it to mention restart", got)
	}
	wantView("7 action", "restart → ok")
}
