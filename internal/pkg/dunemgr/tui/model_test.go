package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateQuitsOnQ(t *testing.T) {
	m := newModel(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("pressing q in navigation mode should return a quit command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("quit command produced nil msg")
	} else if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q should produce tea.QuitMsg, got %T", msg)
	}
}

func TestUpdateCtrlCQuits(t *testing.T) {
	m := newModel(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a quit command")
	}
}

func TestUpdateArrowsMoveSelection(t *testing.T) {
	m := newModel(nil, nil)
	m.hosts = []string{"vm-a", "vm-b", "vm-c"}
	m.statuses = map[string]hostStatus{}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(model).selected != 1 {
		t.Fatalf("down: selected = %d, want 1", m2.(model).selected)
	}
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyUp})
	if m3.(model).selected != 0 {
		t.Fatalf("up: selected = %d, want 0", m3.(model).selected)
	}
	// up at top clamps
	m4, _ := m3.(model).Update(tea.KeyMsg{Type: tea.KeyUp})
	if m4.(model).selected != 0 {
		t.Fatalf("up clamp: selected = %d, want 0", m4.(model).selected)
	}
}

func TestUpdatePollMsgFoldsStatus(t *testing.T) {
	m := newModel(nil, nil)
	m.hosts = []string{"vm-a"}
	m.statuses = map[string]hostStatus{}

	m2, _ := m.Update(pollMsg{host: "vm-a", kind: pollBG, bgState: "RUNNING", ready: 2, total: 2})
	got := m2.(model).statuses["vm-a"]
	if got.bgState != "RUNNING" || got.ready != 2 || got.total != 2 {
		t.Fatalf("bg fold: %+v", got)
	}
	m3, _ := m2.(model).Update(pollMsg{host: "vm-a", kind: pollHealth, reachable: true})
	if !m3.(model).statuses["vm-a"].reachable {
		t.Fatal("health fold: reachable not set")
	}
	m4, _ := m3.(model).Update(pollMsg{host: "vm-a", kind: pollAction, action: "restart", result: "ok"})
	if m4.(model).statuses["vm-a"].lastAction == "" {
		t.Fatal("action fold: lastAction empty")
	}
	if len(m4.(model).events) == 0 {
		t.Fatal("action should append to event log")
	}
}

func TestColonEntersCommandMode(t *testing.T) {
	m := newModel(context.Background(), nil)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	if m2.(model).mode != modeCmd {
		t.Fatal("':' should enter command mode")
	}
}

func TestEscLeavesCommandMode(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeCmd
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m2.(model).mode != modeNav {
		t.Fatal("esc should return to navigation mode")
	}
}

func TestCmdResultMsgRendersInPane(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeCmd
	m2, _ := m.Update(cmdResultMsg{out: "backup created\nkey=abc"})
	mm := m2.(model)
	if mm.mode != modeNav {
		t.Fatal("a command result should return to navigation mode")
	}
	if !strings.Contains(mm.output, "backup created") {
		t.Fatalf("output pane missing result: %q", mm.output)
	}
}

func TestParseLineSplitsArgv(t *testing.T) {
	got := parseLine("  lifecycle vm-a   restart ")
	want := []string{"lifecycle", "vm-a", "restart"}
	if len(got) != len(want) {
		t.Fatalf("parseLine = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseLine[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTabCyclesFocus(t *testing.T) {
	m := newModel(nil, nil)
	initial := m.focus
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m2.(model).focus == initial {
		t.Fatal("tab should change focus pane")
	}
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyTab})
	if m3.(model).focus != initial {
		t.Fatal("tab twice should return to initial focus")
	}
}

func TestTabCompletesVerbInCommandMode(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeCmd
	m.input.SetValue("lifec")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := m2.(model).input.Value(); got != "lifecycle " {
		t.Fatalf("Tab complete = %q, want \"lifecycle \"", got)
	}
}

func TestHelpCommandFillsOutputPane(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeCmd
	m.input.SetValue("help")
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := m2.(model)
	if mm.mode != modeNav {
		t.Fatal("help should return to nav mode")
	}
	if cmd != nil {
		t.Fatal("help is a built-in; it must NOT dispatch (cmd should be nil)")
	}
	for _, want := range []string{"lifecycle", "backup", "shutdown"} {
		if !strings.Contains(mm.output, want) {
			t.Errorf("help output missing %q:\n%s", want, mm.output)
		}
	}
}

func TestCommandHistoryRecall(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeCmd
	m.history = []string{"host list", "lifecycle vm-a restart"}
	m.histIdx = len(m.history)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := m2.(model).input.Value(); got != "lifecycle vm-a restart" {
		t.Fatalf("up recall = %q", got)
	}
}

func TestUsageHintForTypedVerb(t *testing.T) {
	hint := usageHint("give cur")
	if !strings.Contains(hint, "give") {
		t.Fatalf("usage hint missing verb: %q", hint)
	}
}

func TestOutputScrollOffsetClamps(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.output = strings.Repeat("line\n", 100)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m2.(model).outScroll <= 0 {
		t.Fatal("PgDown should advance output scroll")
	}
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m3.(model).outScroll != 0 {
		t.Fatalf("PgUp from one step should return to 0, got %d", m3.(model).outScroll)
	}

	// PgDown far past the end must clamp at maxScroll (lines - window), not run away.
	mm := newModel(context.Background(), nil)
	mm.output = strings.Repeat("line\n", 100) // 100 lines, window 10 → maxScroll 90
	for i := 0; i < 200; i++ {
		next, _ := mm.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		mm = next.(model)
	}
	if mm.outScroll != 90 {
		t.Fatalf("PgDown should clamp at maxScroll=90, got %d", mm.outScroll)
	}
}

func TestRunningSetOnDispatchClearedOnResult(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeCmd
	m.input.SetValue("host list")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m2.(model).running {
		t.Fatal("dispatch should set running=true")
	}
	m3, _ := m2.(model).Update(cmdResultMsg{out: "ok"})
	if m3.(model).running {
		t.Fatal("a result should clear running")
	}
}

func TestHelpDoesNotSetRunning(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeCmd
	m.input.SetValue("help")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(model).running {
		t.Fatal("the help builtin must not set running")
	}
}

// TestRefreshClearsPlayerCache verifies that ":refresh" drops the per-host
// name cache and does NOT dispatch (cmd must be nil, running stays false).
func TestRefreshClearsPlayerCache(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.playerNames = map[string][]string{"vm-a": {"Stilgar"}}
	m.mode = modeCmd
	m.input.SetValue("refresh")
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := m2.(model).playerNames["vm-a"]; ok {
		t.Fatal(":refresh should drop the cache for the selected host")
	}
	if cmd != nil {
		t.Fatal(":refresh must not dispatch (cmd should be nil)")
	}
	if m2.(model).running {
		t.Fatal(":refresh must not set running")
	}
}

// TestPlayerNamesMsgStored verifies that a playerNamesMsg is folded into the
// model's playerNames map.
func TestPlayerNamesMsgStored(t *testing.T) {
	m := newModel(context.Background(), nil)
	m2, _ := m.Update(playerNamesMsg{host: "vm-a", names: []string{"Stilgar"}})
	if got := m2.(model).playerNames["vm-a"]; len(got) != 1 || got[0] != "Stilgar" {
		t.Fatalf("playerNamesMsg not stored: %v", got)
	}
}

func TestViewHasBorders(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.width, m.height = 100, 30
	m.hosts = []string{"vm-a"}
	m.statuses = map[string]hostStatus{"vm-a": {bgState: "Running", ready: 2, total: 2, reachable: true}}
	out := m.View()
	if !strings.Contains(out, "│") || !strings.Contains(out, "─") {
		t.Fatalf("expected lipgloss borders in the framed layout:\n%s", out)
	}
}

func TestViewZeroWidthFallbackUnchanged(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.statuses = map[string]hostStatus{"vm-a": {bgState: "Running", ready: 2, total: 2}}
	out := m.View() // width==0 → plain fallback, no borders required
	if strings.Contains(out, "┌") {
		t.Fatalf("zero-width fallback should stay unbordered:\n%s", out)
	}
}
