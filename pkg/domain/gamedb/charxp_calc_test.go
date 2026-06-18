package gamedb

import "testing"

func TestXPToLevel(t *testing.T) {
	cases := []struct {
		xp   int64
		want int
	}{
		{0, 0}, {40, 1}, {215, 2}, {39, 1}, {344440, 200}, {999999999, 200},
	}
	for _, c := range cases {
		if got := xpToLevel(c.xp); got != c.want {
			t.Errorf("xpToLevel(%d)=%d want %d", c.xp, got, c.want)
		}
	}
}

func TestIntelAtLevel(t *testing.T) {
	cases := []struct {
		lvl  int
		want int64
	}{
		{0, 0}, {1, 4}, {3, 8}, {15, 44}, {30, 119}, {200, 2779},
	}
	for _, c := range cases {
		if got := intelAtLevel(c.lvl); got != c.want {
			t.Errorf("intelAtLevel(%d)=%d want %d", c.lvl, got, c.want)
		}
	}
}

func TestComputeAwardCharXPOutcome(t *testing.T) {
	o := computeAwardCharXPOutcome(0, 0, 0, 40)
	if o.newLevel != 1 || o.newTotalSP != 1 || o.newUnspentSP != 0 {
		t.Fatalf("got %+v", o)
	}
	o = computeAwardCharXPOutcome(maxCharXP, 0, 0, 1000)
	if !o.capped || o.newXP != maxCharXP {
		t.Fatalf("cap not honored: %+v", o)
	}
}
