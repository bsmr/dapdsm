package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
	admincatalog "go.muehmer.eu/dapdsm/pkg/domain/catalog"
	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
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

// TestUpdateArrowsMoveSelection verifies that up/down move the nav cursor.
// (Left/right now descend/ascend the nav tree, not host selection.)
func TestUpdateArrowsMoveSelection(t *testing.T) {
	m := newModel(nil, nil)
	m.hosts = []string{"vm-a", "vm-b", "vm-c"}
	m.nav.counts[levelHosts] = 3
	m.statuses = map[string]hostStatus{}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(model).nav.sel[levelHosts] != 1 {
		t.Fatalf("down: nav.sel[hosts]=%d want 1", m2.(model).nav.sel[levelHosts])
	}
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyDown})
	if m3.(model).nav.sel[levelHosts] != 2 {
		t.Fatalf("down×2: nav.sel[hosts]=%d want 2", m3.(model).nav.sel[levelHosts])
	}
	m4, _ := m3.(model).Update(tea.KeyMsg{Type: tea.KeyUp})
	if m4.(model).nav.sel[levelHosts] != 1 {
		t.Fatalf("up: nav.sel[hosts]=%d want 1", m4.(model).nav.sel[levelHosts])
	}
}

// TestArrowKeysDriveNavCursor replaces the old host-cycling test.
// Right descends and must return a load cmd; left ascends back to hosts.
func TestArrowKeysDriveNavCursor(t *testing.T) {
	m := newModel(nil, nil)
	m.hosts = []string{"vm-a", "vm-b", "vm-c"}
	m.nav.counts[levelHosts] = 3

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m2.(model).nav.level != levelPlayers {
		t.Fatalf("right: level=%v want players", m2.(model).nav.level)
	}
	if cmd == nil {
		t.Fatal("descend to players must return a load cmd")
	}

	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m3.(model).nav.level != levelHosts {
		t.Fatalf("left: level=%v want hosts", m3.(model).nav.level)
	}
}

func TestHeaderContentFooterOrder(t *testing.T) {
	m := newModel(nil, nil)
	m.hosts = []string{"vm-a"}
	m.nav.counts[levelHosts] = 1
	m.width, m.height = 100, 30
	m.output = "RESULT-LINE"
	m.mode = modeCmd
	m.input.Prompt = "[vm-a] › "
	m.input.SetValue("whisper x")
	v := m.View()
	hi := strings.Index(v, "vm-a")
	ci := strings.Index(v, "RESULT-LINE")
	ri := strings.Index(v, "›")
	if !(hi >= 0 && ci > hi && ri > ci) {
		t.Fatalf("order header<content<footer broken: hi=%d ci=%d ri=%d\n%s", hi, ci, ri, v)
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

// TestOutputScrollOffsetClamps verifies that PgUp/PgDn in nav mode move the
// nav cursor (not the output scroll offset), clamped at list bounds.
func TestOutputScrollOffsetClamps(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.counts[levelHosts] = 1

	// PgDown on a 1-item list: cursor stays at 0 (clamps), but the key is handled.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m2.(model).nav.sel[levelHosts] != 0 {
		t.Fatalf("PgDown on single-item list: sel=%d want 0", m2.(model).nav.sel[levelHosts])
	}

	// PgUp on 0: stays at 0.
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m3.(model).nav.sel[levelHosts] != 0 {
		t.Fatalf("PgUp from 0: sel=%d want 0", m3.(model).nav.sel[levelHosts])
	}

	// With many hosts PgDown advances the cursor.
	mm := newModel(context.Background(), nil)
	for i := 0; i < 50; i++ {
		mm.hosts = append(mm.hosts, "vm-x")
	}
	mm.nav.counts[levelHosts] = len(mm.hosts)
	next, _ := mm.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if next.(model).nav.sel[levelHosts] <= 0 {
		t.Fatalf("PgDown should advance cursor on multi-item list, got sel=%d", next.(model).nav.sel[levelHosts])
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
	m.nav.counts[levelHosts] = 1
	m.statuses = map[string]hostStatus{"vm-a": {bgState: "Running", ready: 2, total: 2, reachable: true}}
	out := m.View()
	if !strings.Contains(out, "│") || !strings.Contains(out, "─") {
		t.Fatalf("expected lipgloss borders in the framed layout:\n%s", out)
	}
}

func TestViewZeroWidthFallbackUnchanged(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.counts[levelHosts] = 1
	m.statuses = map[string]hostStatus{"vm-a": {bgState: "Running", ready: 2, total: 2}}
	out := m.View() // width==0 → plain fallback, no borders required
	if strings.Contains(out, "┌") {
		t.Fatalf("zero-width fallback should stay unbordered:\n%s", out)
	}
}

// TestNavFooterHasNoFocus verifies that the nav footer no longer advertises the
// removed [tab] focus affordance.
func TestNavFooterHasNoFocus(t *testing.T) {
	m := newModel(nil, nil)
	m.width = 100
	m.height = 30
	if strings.Contains(m.View(), "focus") {
		t.Errorf("nav footer still mentions focus:\n%s", m.View())
	}
}

// TestItemVerbDispatchesFromBar verifies that the "item" verb is no longer
// intercepted as a TUI built-in and instead falls through to the normal
// command dispatcher (sets running=true and returns a non-nil tea.Cmd).
func TestItemVerbDispatchesFromBar(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.mode = modeCmd
	m.input.SetValue("item vm-a set 8841 --qty 50 --confirm")
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m2.(model).running || cmd == nil {
		t.Fatal("item verb must dispatch from the command bar (not be intercepted)")
	}
}

// TestPlayerCommandNonInspectDispatches verifies that a non-inspect player
// command falls through to the normal dispatch path (sets running and returns
// a tea.Cmd).
func TestPlayerCommandNonInspectDispatches(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.mode = modeCmd
	m.input.SetValue("player vm-a search foo")
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m2.(model).running {
		t.Fatal("non-inspect player command must set running=true")
	}
	if cmd == nil {
		t.Fatal("non-inspect player command must return a dispatch tea.Cmd")
	}
}

// TestNavKeysDriveLevels verifies that the modal nav keys move through levels.
func TestNavKeysDriveLevels(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a", "vm-b"}
	m.nav.counts[levelHosts] = 2
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(model).nav.sel[levelHosts] != 1 {
		t.Fatalf("down: hosts sel=%d want 1", m2.(model).nav.sel[levelHosts])
	}
	m3, cmd := m2.(model).Update(tea.KeyMsg{Type: tea.KeyRight})
	if m3.(model).nav.level != levelPlayers {
		t.Fatalf("right: level=%v want players", m3.(model).nav.level)
	}
	if cmd == nil {
		t.Fatal("descend to players must return a load cmd")
	}
	m4, _ := m3.(model).Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m4.(model).nav.level != levelHosts {
		t.Fatalf("left: level=%v want hosts", m4.(model).nav.level)
	}
	m5, _ := m4.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	if m5.(model).mode != modeCmd {
		t.Fatal("':' must enter command mode")
	}
}

func TestCommandOutputScrolls(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.height = 12
	m.output = strings.Join([]string{"l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12"}, "\n")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(model).outScroll != 1 {
		t.Fatalf("down on output: outScroll=%d want 1", m2.(model).outScroll)
	}
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m3.(model).output != "" {
		t.Fatal("left must dismiss command output")
	}
}

func TestLoadFailureSetsNavErr(t *testing.T) {
	m := newModel(context.Background(), nil)
	m2, _ := m.Update(playersMsg{err: "load players failed"})
	if m2.(model).navErr == "" {
		t.Fatal("playersMsg with err must set navErr")
	}
}

func TestSelectedItemID(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.items = []dbquery.ItemRow{{ID: 7}, {ID: 8}}
	m.nav.level = levelItem
	m.nav.sel[levelItem] = 1
	if got := m.selectedItemID(); got != 8 {
		t.Fatalf("selectedItemID=%d want 8", got)
	}
}

func TestItemDeleteConfirmFlow(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 8841, TemplateID: "Ammo", StackSize: 10}}
	m.nav.counts[levelItem] = 1
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !m2.(model).confirmDelete {
		t.Fatal("'d' must enter delete-confirm")
	}
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m3.(model).confirmDelete {
		t.Fatal("'n' must cancel delete-confirm")
	}
}

func TestItemEditOnlineOwnerShowsNavErr(t *testing.T) {
	m := newModel(context.Background(), nil)
	m2, _ := m.Update(editDoneMsg{err: command.ErrItemOwnerOnline})
	if m2.(model).navErr == "" {
		t.Fatal("online-owner edit must surface navErr")
	}
}

// TestSelectedHostFollowsCursor verifies that selectedHost() reflects the nav
// cursor immediately after a move — no descend() required.
func TestSelectedHostFollowsCursor(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a", "vm-b"}
	m.nav.counts[levelHosts] = 2
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := m2.(model).selectedHost(); got != "vm-b" {
		t.Fatalf("selectedHost=%q want vm-b (cursor follows)", got)
	}
}

// TestItemsReloadKeepsCursor verifies that an edit-reload clamps the existing
// item cursor rather than resetting it to 0.
func TestItemsReloadKeepsCursor(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	m.nav.sel[levelItem] = 2
	m2, _ := m.Update(itemsMsg{items: []dbquery.ItemRow{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}})
	if got := m2.(model).nav.sel[levelItem]; got != 2 {
		t.Fatalf("sel=%d want 2 (kept on reload)", got)
	}
	// shrink → clamp
	m3, _ := m2.(model).Update(itemsMsg{items: []dbquery.ItemRow{{ID: 1}}})
	if got := m3.(model).nav.sel[levelItem]; got != 0 {
		t.Fatalf("sel=%d want 0 (clamped after shrink)", got)
	}
}

func TestViewFillsHeight(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.counts[levelHosts] = 1
	m.width, m.height = 80, 24
	got := lipgloss.Height(m.View())
	if got < 23 || got > 24 {
		t.Fatalf("View height=%d, want ≈24 (fills the screen)", got)
	}
}

// TestDescendShowsLoadingNotStale verifies that descend() clears stale data and
// sets loading=true, and that the matching load msg clears loading again.
func TestDescendShowsLoadingNotStale(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.counts[levelHosts] = 1
	m.players = []dbquery.Player{{CharacterName: "Stale"}}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	mm := m2.(model)
	if !mm.loading {
		t.Fatal("descend must set loading=true")
	}
	if len(mm.players) != 0 {
		t.Fatalf("stale players not cleared: %v", mm.players)
	}
	m3, _ := mm.Update(playersMsg{players: []dbquery.Player{{CharacterName: "Real"}}})
	if m3.(model).loading {
		t.Fatal("playersMsg must clear loading")
	}
}

func TestItemEditRejectsNonNumeric(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 1, StackSize: 5}}
	m.nav.counts[levelItem] = 1
	m.editing, m.editKind = true, editKindQty
	m.input.SetValue("abc")
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(model).navErr == "" {
		t.Fatal("non-numeric qty must set navErr, not apply")
	}
	if m2.(model).running {
		t.Fatal("non-numeric qty must not start an apply")
	}
	_ = cmd
}

func TestModeIndicatorAndHelpOverlay(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.counts[levelHosts] = 1
	if !strings.Contains(m.breadcrumb(), "[NAV]") {
		t.Fatalf("breadcrumb missing [NAV]: %q", m.breadcrumb())
	}
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m2.(model).showHelp {
		t.Fatal("'?' must open help overlay")
	}
	m3, _ := m2.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m3.(model).showHelp {
		t.Fatal("a key after '?' must close the overlay")
	}
}

func TestItemRowsUseDisplayName(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 1, TemplateID: "Ammo", StackSize: 5}}
	rows := m.levelRows()
	if len(rows) != 1 || !strings.Contains(rows[0], admincatalog.DisplayName("Ammo")) {
		t.Fatalf("item row must use the display name: %v", rows)
	}
}

func TestStackClampAndRowMax(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 1, TemplateID: "Radiation_Suit", StackSize: 1}} // stack_max 1
	m.nav.counts[levelItem] = 1
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	if cmd != nil || m2.(model).running {
		t.Fatal("'+' at stack_max must be a no-op")
	}
	if m2.(model).navErr == "" {
		t.Fatal("'+' at max should explain via navErr")
	}
	// New column model: STACK and MAX are separate right-aligned columns.
	rows := m.levelRows()
	if !strings.Contains(rows[0], "1") {
		t.Fatalf("row should show stack size 1: %q", rows[0])
	}
}

func TestEditQtyRejectsOverMax(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 1, TemplateID: "Radiation_Suit", StackSize: 1}}
	m.nav.counts[levelItem] = 1
	m.editing, m.editKind = true, editKindQty
	m.input.SetValue("5")
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m2.(model).running {
		t.Fatal("over-max qty must not apply")
	}
	if m2.(model).navErr == "" {
		t.Fatal("over-max qty must set navErr")
	}
}

func TestAddItemKeyPrefillsCommandBar(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.level = levelInventory
	m.curChar = "Mal"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	mm := m2.(model)
	if mm.mode != modeCmd {
		t.Fatal("'a' must enter command mode")
	}
	if !strings.HasPrefix(mm.input.Value(), "give item Mal ") {
		t.Fatalf("prefill = %q want 'give item Mal '", mm.input.Value())
	}
}

func TestAddItemKeyAtPlayersLevel(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.hosts = []string{"vm-a"}
	m.nav.level = levelPlayers
	m.players = []dbquery.Player{{CharacterName: "Chani"}}
	m.nav.counts[levelPlayers] = 1
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !strings.HasPrefix(m2.(model).input.Value(), "give item Chani ") {
		t.Fatalf("prefill = %q", m2.(model).input.Value())
	}
}

func TestCmdResultUsageErrorNotDoubled(t *testing.T) {
	m := newModel(context.Background(), nil)
	m2, _ := m.Update(cmdResultMsg{
		out: "admin skillpoints: --points N required (or use ...)\n",
		err: fmt.Errorf("admin skillpoints: --points required: %w", command.ErrUsage),
	})
	out := m2.(model).output
	if strings.Contains(out, "error:") {
		t.Fatalf("usage error must not append a second 'error:' line:\n%s", out)
	}
	if !strings.Contains(out, "--points N required") {
		t.Fatalf("the helpful stderr message must remain:\n%s", out)
	}
}

func TestCmdResultNonUsageErrorStillAppended(t *testing.T) {
	m := newModel(context.Background(), nil)
	m2, _ := m.Update(cmdResultMsg{out: "", err: fmt.Errorf("ssh: connection refused")})
	if !strings.Contains(m2.(model).output, "error: ssh: connection refused") {
		t.Fatalf("non-usage error should still be shown, got %q", m2.(model).output)
	}
}

func TestLevelHeaderPerLevel(t *testing.T) {
	m := newModel(context.Background(), nil)

	m.nav.level = levelHosts
	if h := m.levelHeader(); h != "" {
		t.Fatalf("hosts level has no header, got %q", h)
	}

	m.nav.level = levelPlayers
	if h := m.levelHeader(); !strings.Contains(h, "NAME") || !strings.Contains(h, "STATUS") {
		t.Fatalf("players header = %q", h)
	}

	m.nav.level = levelInventory
	if h := m.levelHeader(); !strings.Contains(h, "TYPE") || !strings.Contains(h, "ITEMS") {
		t.Fatalf("inventory header = %q", h)
	}

	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 1, TemplateID: "ZZZ_A", StackSize: 1}}
	if h := m.levelHeader(); !strings.Contains(h, "ID") || !strings.Contains(h, "NAME") || !strings.Contains(h, "STACK") {
		t.Fatalf("item header = %q", h)
	}
}

func TestItemHeaderColumnsPresent(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 1, TemplateID: "ZZZ_A", StackSize: 1}}
	h := m.levelHeader()
	for _, lbl := range []string{"ID", "NAME", "STACK", "MAX", "Q"} {
		if !strings.Contains(h, lbl) {
			t.Fatalf("item header missing %q: %q", lbl, h)
		}
	}
}

func TestItemStackRightAlignedNoMaxBlank(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	// ZZZ_* are non-catalog → StackMax 0 (blank MAX), DisplayName = id string.
	m.items = []dbquery.ItemRow{
		{ID: 1, TemplateID: "ZZZ_AAAAAAAAAAAA", StackSize: 7},
		{ID: 2, TemplateID: "ZZZ_BBBBBBBBBBBB", StackSize: 1234},
	}
	rows := m.levelRows()
	end := func(s, sub string) int { return strings.Index(s, sub) + len(sub) }
	if end(rows[0], "7") != end(rows[1], "1234") {
		t.Fatalf("STACK not right-aligned:\n%q\n%q", rows[0], rows[1])
	}
	if strings.Contains(rows[0], "/") {
		t.Fatalf("blank max must not render a slash: %q", rows[0])
	}
}

func TestItemHeaderAlignsWithRowColumns(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{
		{ID: 1494806, TemplateID: "ZZZ_TMPL_AAA", StackSize: 1},
		{ID: 1496771, TemplateID: "ZZZ_TMPL_BBB", StackSize: 12345678},
	}
	header := m.levelHeader()
	rows := m.levelRows()
	off := strings.Index(header, "NAME")
	for _, r := range rows {
		if got := strings.Index(r, "ZZZ_TMPL_"); got != off {
			t.Fatalf("NAME offset %d != header %d\n%q\n%q", got, off, header, r)
		}
	}
}

func TestOutputHeaderPinned(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.height = 24
	m.outHeaderLines = 1
	m.output = "NAME    FLS\nrow1\nrow2\nrow3\nrow4\nrow5"
	m.outScroll = 2 // scrolled into the data
	body := m.renderOutputBody(contentWindow(m.height))
	first := strings.SplitN(body, "\n", 2)[0]
	if !strings.Contains(first, "NAME") {
		t.Fatalf("header must stay pinned at row 0 while scrolled, got first line %q", first)
	}
}

func TestOutputHeaderClearedOnError(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.outHeaderLines = 1
	m2, _ := m.Update(cmdResultMsg{out: "boom\n", err: fmt.Errorf("ssh: down")})
	if m2.(model).outHeaderLines != 0 {
		t.Fatalf("an errored command must clear outHeaderLines, got %d", m2.(model).outHeaderLines)
	}
	// success keeps whatever was set at dispatch
	m3 := newModel(context.Background(), nil)
	m3.outHeaderLines = 1
	m4, _ := m3.Update(cmdResultMsg{out: "NAME\nrow\n"})
	if m4.(model).outHeaderLines != 1 {
		t.Fatalf("a successful command must keep outHeaderLines, got %d", m4.(model).outHeaderLines)
	}
}

func TestFooterShowsDismissHintOnOutput(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.width, m.height = 100, 24
	m.output = "error: grant item: count 2000 out of range (1..1000)"
	view := m.View()
	if !strings.Contains(view, "←/Esc back") {
		t.Fatalf("output view footer must show a dismiss hint (←/Esc back):\n%s", view)
	}
}

func names(ps []dbquery.Player) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.CharacterName
	}
	return out
}

func TestSortPlayersByNameAscDesc(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelPlayers
	m.players = []dbquery.Player{
		{CharacterName: "Charlie"}, {CharacterName: "alice"}, {CharacterName: "Bob"},
	}
	m.applySort()
	if m.players[0].CharacterName != "alice" || m.players[2].CharacterName != "Charlie" {
		t.Fatalf("ascending case-insensitive name sort wrong: %v", names(m.players))
	}
	m.sortDesc[levelPlayers] = true
	m.applySort()
	if m.players[0].CharacterName != "Charlie" || m.players[2].CharacterName != "alice" {
		t.Fatalf("descending wrong: %v", names(m.players))
	}
}

func TestSortItemsByStackNumeric(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	m.sortCol[levelItem] = 2 // STACK
	m.items = []dbquery.ItemRow{
		{ID: 1, StackSize: 44}, {ID: 2, StackSize: 1000}, {ID: 3, StackSize: 7},
	}
	m.applySort()
	if m.items[0].StackSize != 7 || m.items[2].StackSize != 1000 {
		t.Fatalf("numeric stack sort wrong: %v", m.items)
	}
}

func TestSKeyCyclesSortableColumnsSkippingMax(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelItem
	m.items = []dbquery.ItemRow{{ID: 1, StackSize: 1}}
	seen := map[int]bool{}
	for i := 0; i < 4; i++ {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		m = m2.(model)
		if m.sortCol[levelItem] == 3 {
			t.Fatalf("'s' must skip the non-sortable MAX column (3)")
		}
		seen[m.sortCol[levelItem]] = true
	}
	if !seen[0] || !seen[2] || !seen[4] {
		t.Fatalf("'s' should reach ID/STACK/Q over cycles, saw %v", seen)
	}
}

func TestShiftSKeyTogglesDirection(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelPlayers
	m.players = []dbquery.Player{{CharacterName: "a"}, {CharacterName: "b"}}
	before := m.sortDesc[levelPlayers]
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if m2.(model).sortDesc[levelPlayers] == before {
		t.Fatal("'S' must toggle sort direction")
	}
}

func TestPlayersMsgLandsSorted(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.nav.level = levelPlayers
	m2, _ := m.Update(playersMsg{players: []dbquery.Player{
		{CharacterName: "zoe"}, {CharacterName: "amy"},
	}})
	if m2.(model).players[0].CharacterName != "amy" {
		t.Fatalf("a fresh playersMsg must land sorted, got %v", names(m2.(model).players))
	}
}
