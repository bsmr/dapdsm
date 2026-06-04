package command

import "testing"

func TestListingHeaderLines(t *testing.T) {
	cases := []struct {
		argv []string
		want int
	}{
		{[]string{"player", "vm-a", "search"}, 1},
		{[]string{"avatar", "vm-a", "list"}, 1},
		{[]string{"avatar", "vm-a", "exports"}, 1},
		{[]string{"avatar", "vm-a", "export", "x"}, 0},
		{[]string{"player", "vm-a", "pos", "x"}, 0},
		{[]string{"item", "vm-a", "set", "1"}, 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := ListingHeaderLines(c.argv); got != c.want {
			t.Errorf("ListingHeaderLines(%v)=%d want %d", c.argv, got, c.want)
		}
	}
}
