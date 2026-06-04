package tui

import (
	"fmt"
	"strings"
	"testing"

	lipgloss "github.com/charmbracelet/lipgloss"
)

func TestRenderSuggestionsAllFitNoOverflow(t *testing.T) {
	cands := []string{"admin", "avatar", "backup", "broadcast", "db", "give",
		"help", "host", "ini", "item", "lifecycle", "player", "reconcile", "stats", "whisper"}
	out := renderSuggestions(cands, 80, 4)
	if strings.Contains(out, "more") {
		t.Fatalf("15 verbs fit in 4 rows; unexpected overflow marker:\n%s", out)
	}
	for _, c := range cands {
		if !strings.Contains(out, c) {
			t.Fatalf("missing candidate %q in:\n%s", c, out)
		}
	}
	if rows := strings.Count(out, "\n") + 1; rows > 4 {
		t.Fatalf("rows=%d want <=4:\n%s", rows, out)
	}
}

func TestRenderSuggestionsOverflowMarker(t *testing.T) {
	cands := make([]string, 500)
	for i := range cands {
		cands[i] = fmt.Sprintf("item%03d", i)
	}
	out := renderSuggestions(cands, 80, 4)
	if rows := strings.Count(out, "\n") + 1; rows > 4 {
		t.Fatalf("rows=%d want <=4", rows)
	}
	if !strings.Contains(out, "more)") {
		t.Fatalf("large set must show overflow marker:\n%s", out)
	}
}

func TestRenderSuggestionsEmpty(t *testing.T) {
	if got := renderSuggestions(nil, 80, 4); got != "" {
		t.Fatalf("empty candidates → empty string, got %q", got)
	}
}

func TestRenderViewIncludesAllPanes(t *testing.T) {
	m := newModel(nil, nil)
	m.hosts = []string{"vm-a"}
	m.nav.counts[levelHosts] = 1
	m.statuses = map[string]hostStatus{"vm-a": {bgState: "RUNNING", ready: 2, total: 2, reachable: true}}
	// Events are not auto-displayed in the nav list; put result into output pane.
	m.output = "restart → ok"
	m.width, m.height = 80, 24
	v := m.View()
	for _, want := range []string{"vm-a", "RUNNING", "restart → ok"} {
		if !strings.Contains(v, want) {
			t.Errorf("View missing %q:\n%s", want, v)
		}
	}
}

func TestRenderListViewportAndCursor(t *testing.T) {
	rows := []string{"a", "b", "c", "d", "e"}
	out := renderList(rows, 4 /*selected*/, 3 /*height*/)
	if !strings.Contains(out, "▸") || !strings.Contains(out, "e") {
		t.Fatalf("cursor/selected not shown:\n%s", out)
	}
	// "a" must be scrolled out of a height-3 window when the cursor is at the end.
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "a" {
			t.Fatalf("row a should be scrolled out:\n%s", out)
		}
	}
}

func TestRenderListEmpty(t *testing.T) {
	if got := renderList(nil, 0, 5); !strings.Contains(got, "empty") {
		t.Fatalf("empty list = %q", got)
	}
}

func TestTitledBox(t *testing.T) {
	out := framedBox("Hosts", "body-text", 40, 0)
	if !strings.Contains(out, "Hosts") || !strings.Contains(out, "body-text") {
		t.Fatalf("titled box missing title/body:\n%s", out)
	}
}

func TestFramedBoxTitleInBorder(t *testing.T) {
	out := framedBox("Hosts", "body", 30, 0)
	lines := strings.Split(out, "\n")
	top := lines[0]
	if !strings.Contains(top, "Hosts") {
		t.Fatalf("title not in top border:\n%s", out)
	}
	if !strings.HasPrefix(top, "╭") {
		t.Fatalf("top line is not a border edge: %q", top)
	}
	if !strings.Contains(out, "body") {
		t.Fatalf("body missing:\n%s", out)
	}
	long := framedBox(strings.Repeat("X", 100), "b", 20, 0)
	topw := lipgloss.Width(strings.Split(long, "\n")[0])
	if topw > 22 { // box width(20) + 2 corners; over-long title must not overflow
		t.Fatalf("over-long title overflowed: top width %d", topw)
	}
}

func TestFramedBoxFixedHeight(t *testing.T) {
	out := framedBox("T", "one line", 20, 8)
	if h := lipgloss.Height(out); h != 8 {
		t.Fatalf("framedBox height=%d want 8", h)
	}
}

func TestRenderHeaderShowsBadges(t *testing.T) {
	st := map[string]hostStatus{
		"vm-a": {bgState: "Running", ready: 3, total: 3, reachable: true},
		"vm-b": {bgState: "Stopped", ready: 0, total: 3, reachable: false},
	}
	out := renderHeader([]string{"vm-a", "vm-b"}, st, 0)
	if !strings.Contains(out, "●") || !strings.Contains(out, "○") {
		t.Fatalf("expected reachable/unreachable badges:\n%s", out)
	}
}

func TestRenderHeaderShowsAllHostsActiveMarked(t *testing.T) {
	hosts := []string{"vm-dune-01", "vm-dune-02"}
	st := map[string]hostStatus{
		"vm-dune-01": {bgState: "RUNNING", ready: 2, total: 2, reachable: true},
		"vm-dune-02": {bgState: "DEGRADED", ready: 1, total: 2, reachable: false},
	}
	out := renderHeader(hosts, st, 0)
	if !strings.Contains(out, "vm-dune-01") || !strings.Contains(out, "vm-dune-02") {
		t.Fatalf("header missing a host:\n%s", out)
	}
	if !strings.Contains(out, "▸") {
		t.Fatalf("active host not marked:\n%s", out)
	}
}

func TestRenderSuggestionsNarrowSingleColumn(t *testing.T) {
	cands := []string{"alpha", "bravo", "charlie", "delta"}
	out := renderSuggestions(cands, 5, 3) // width < cell → one column
	rows := strings.Split(strings.TrimRight(out, " "), "\n")
	if len(rows) > 3 {
		t.Fatalf("must cap at maxRows=3, got %d rows:\n%s", len(rows), out)
	}
	if !strings.Contains(out, "more)") {
		t.Fatalf("4 candidates in a 3-row single column must overflow:\n%s", out)
	}
}

func TestHelpTextCoversModes(t *testing.T) {
	h := helpText()
	for _, want := range []string{"command mode", "drill", "+/-"} {
		if !strings.Contains(h, want) {
			t.Fatalf("helpText missing %q", want)
		}
	}
}

func TestWrapBreakAtSpacesAndPipe(t *testing.T) {
	in := "admin <host> [clean|item|kick|reset|skill|skillpoints|teleport|vehicle|water|xp]"
	lines := wrapBreak(in, 30)
	for _, ln := range lines {
		if lipgloss.Width(ln) > 30 {
			t.Fatalf("line exceeds width 30: %q", ln)
		}
	}
	if len(lines) < 2 {
		t.Fatalf("long usage should wrap to multiple lines, got %v", lines)
	}
}

func TestWrapBreakHardBreakAndNarrow(t *testing.T) {
	// token longer than width with no break char → hard-break into width-sized chunks
	got := wrapBreak("toolongtoken", 5)
	for _, ln := range got {
		if len(ln) > 5 {
			t.Fatalf("hard-break must respect width 5: %q", ln)
		}
	}
	if len(got) < 2 {
		t.Fatalf("a 12-char token must hard-break at width 5, got %v", got)
	}
	// width=1 must still terminate and never produce >1-char lines
	w1 := wrapBreak("abc", 1)
	if len(w1) != 3 {
		t.Fatalf("width=1 on 'abc' → 3 single-char lines, got %v", w1)
	}
}

func TestWrapHelperOverflowMarker(t *testing.T) {
	in := "l1\nl2\nl3\nl4\nl5\nl6"
	got := wrapHelper(in, 40, 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("must cap at maxRows=3, got %d:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[2], "more)") {
		t.Fatalf("last line must be the overflow marker, got %q", lines[2])
	}
}
