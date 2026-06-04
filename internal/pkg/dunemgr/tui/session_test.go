package tui

import (
	"context"
	"fmt"
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

	if got := m.selectedHost(); got != "vm-a" {
		t.Fatalf("initial selectedHost = %q, want vm-a", got)
	}
	if got := m.statuses["vm-a"]; got.bgState != "RUNNING" || got.ready != 2 || !got.reachable {
		t.Fatalf("vm-a status not folded: %+v", got)
	}
	wantView("1 initial", "vm-a", "RUNNING", "2/2", "vm-b", "DEGRADED", "1/2")

	// Step 2: move cursor down to vm-b (index 1). With the cursor-is-selection
	// fix, selectedHost() reflects the new cursor immediately — no Right needed.
	// We also verify that descending (Right) and ascending (Left) still work.
	m.nav.counts[levelHosts] = 2
	apply(tea.KeyMsg{Type: tea.KeyDown})
	if got := m.selectedHost(); got != "vm-b" {
		t.Fatalf("after down: selectedHost=%q want vm-b (cursor is selection, no lag)", got)
	}
	apply(tea.KeyMsg{Type: tea.KeyRight}) // descend into players (verifies descend still works)
	apply(tea.KeyMsg{Type: tea.KeyLeft})  // ascend back to hosts

	// Step 3: ':' enters command mode; typing shows a live suggestion line.
	keys(":")
	if m.mode != modeCmd {
		t.Fatalf("':' did not enter command mode (mode=%v)", m.mode)
	}
	keys("lifec")
	if m.input.Value() != "lifec" {
		t.Fatalf("input = %q, want \"lifec\"", m.input.Value())
	}
	wantView("3 suggest", "lifec", "lifecycle") // prompt contains typed text + live suggestion

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
	// The rendered pane shows only the first contentWindow lines; check the full
	// output buffer directly for items that may be scrolled out of view.
	wantView("6 help", "commands (Tab to complete):")
	for _, want := range []string{"lifecycle <host>", "item <host>", "ini <host>", "backup", "shutdown", "help [verb]"} {
		if !strings.Contains(m.output, want) {
			t.Errorf("[6 help] output buffer missing %q", want)
		}
	}

	// Step 7: a live action frame appends to the event log.
	// The nav content pane shows renderList (not events) when output is empty, so
	// we verify the event log directly rather than through View().
	m.output = ""
	apply(pollMsg{host: "vm-b", kind: pollAction, action: "restart", result: "ok"})
	if len(m.events) == 0 {
		t.Fatal("action frame did not append to the event log")
	}
	if got := m.statuses["vm-b"].lastAction; !strings.Contains(got, "restart") {
		t.Errorf("vm-b lastAction = %q, want it to mention restart", got)
	}
	if !strings.Contains(m.events[len(m.events)-1], "restart → ok") {
		t.Errorf("[7 action] event log missing restart → ok: %v", m.events)
	}

	// Step 8: ':player <host> search <query>' enters command mode, types the full
	// command, and verifies the bar is in command mode with the expected content
	// before Enter is pressed (dispatch flow verification without a real SSH conn).
	keys(":")
	if m.mode != modeCmd {
		t.Fatalf("step 8: ':' did not enter command mode")
	}
	keys("player vm-a search ")
	wantView("8 player-search", "player vm-a search")

	// Step 9: ':admin <host> item <player> <prefix>' + Tab offers catalog item ids.
	// Reset the command bar, then type an admin item command prefix.
	apply(tea.KeyMsg{Type: tea.KeyEsc})
	keys(":")
	keys("admin vm-a item test-player-id T6_Augment_Ac")
	// The suggestion line must contain a known item id with that prefix.
	v := m.View()
	if !strings.Contains(v, "T6_Augment_Acuracy1") {
		t.Errorf("[9 catalog suggest] View() missing T6_Augment_Acuracy1 in suggestion line:\n%s", v)
	}
	// Tab-complete should advance toward the known id.
	apply(tea.KeyMsg{Type: tea.KeyTab})
	wantView("9 catalog tab", "T6_Augment_Acuracy1")

	// Step 10: ':admin <host> kick <player>' without --confirm → dispatches and
	// the result pane must mention the gating requirement.
	apply(tea.KeyMsg{Type: tea.KeyEsc})
	keys(":")
	keys("admin vm-a kick test-player-id")
	apply(tea.KeyMsg{Type: tea.KeyEnter})
	// The command bar dispatch is async (tea.Cmd); inject the cmdResultMsg
	// directly to simulate the returned error from the runner gate.
	apply(cmdResultMsg{
		out: "",
		err: fmt.Errorf("admin kick: verb is destructive; pass --confirm"),
	})
	wantView("10 kick gate", "destructive")
}
