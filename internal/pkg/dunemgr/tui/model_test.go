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
