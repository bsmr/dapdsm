package lifecycle

import "testing"

func TestActionValid(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"start", true},
		{"stop", true},
		{"restart", true},
		{"update", true},
		{"", false},
		{"START", false},
		{"halt", false},
		{"start ", false},
	}
	for _, c := range cases {
		got := Action(c.in).Valid()
		if got != c.want {
			t.Errorf("Action(%q).Valid()=%v, want %v", c.in, got, c.want)
		}
	}
}

func TestValidateAction(t *testing.T) {
	if _, err := ValidateAction("start"); err != nil {
		t.Errorf("ValidateAction(start) err=%v, want nil", err)
	}
	if _, err := ValidateAction("nope"); err == nil {
		t.Error("ValidateAction(nope) err=nil, want non-nil")
	}
}
