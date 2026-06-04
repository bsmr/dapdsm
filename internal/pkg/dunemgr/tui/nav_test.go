package tui

import "testing"

func TestVisibleWindow(t *testing.T) {
	cases := []struct{ sel, wantStart, wantEnd int }{
		{0, 0, 4}, {2, 0, 4}, {3, 0, 4}, {4, 1, 5}, {9, 6, 10},
	}
	for _, c := range cases {
		s, e := visibleWindow(10, c.sel, 4)
		if s != c.wantStart || e != c.wantEnd {
			t.Errorf("visibleWindow(10,%d,4)=(%d,%d) want (%d,%d)", c.sel, s, e, c.wantStart, c.wantEnd)
		}
	}
	if s, e := visibleWindow(3, 1, 10); s != 0 || e != 3 {
		t.Errorf("short list = (%d,%d) want (0,3)", s, e)
	}
	if s, e := visibleWindow(0, 0, 5); s != 0 || e != 0 {
		t.Errorf("empty = (%d,%d) want (0,0)", s, e)
	}
}

func TestNavMoveClamps(t *testing.T) {
	ns := &navState{level: levelItem}
	ns.counts[levelItem] = 3
	ns.move(1)
	ns.move(1)
	ns.move(1)
	if ns.sel[levelItem] != 2 {
		t.Fatalf("sel=%d want 2 (clamped)", ns.sel[levelItem])
	}
	ns.move(-5)
	if ns.sel[levelItem] != 0 {
		t.Fatalf("sel=%d want 0", ns.sel[levelItem])
	}
}

func TestNavDescendAscend(t *testing.T) {
	ns := &navState{level: levelHosts}
	ns.counts[levelHosts] = 2
	ns.descend()
	if ns.level != levelPlayers {
		t.Fatalf("level=%v want players", ns.level)
	}
	ns.descend()
	ns.descend()
	ns.descend() // clamp at item
	if ns.level != levelItem {
		t.Fatalf("level=%v want item (clamped)", ns.level)
	}
	ns.ascend()
	if ns.level != levelInventory {
		t.Fatalf("ascend level=%v want inventory", ns.level)
	}
}

func TestNavJump(t *testing.T) {
	ns := &navState{level: levelPlayers}
	ns.counts[levelPlayers] = 5
	ns.jump(true)
	if ns.sel[levelPlayers] != 4 {
		t.Fatalf("jump end sel=%d want 4", ns.sel[levelPlayers])
	}
	ns.jump(false)
	if ns.sel[levelPlayers] != 0 {
		t.Fatalf("jump home sel=%d want 0", ns.sel[levelPlayers])
	}
}
