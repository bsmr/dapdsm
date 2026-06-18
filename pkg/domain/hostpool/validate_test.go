package hostpool

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"vm-a", true},
		{"vm-host-01", true},
		{"a", true},
		{"", false},
		{"vm with space", false},
		{"-leading-dash", false},
		{"trailing-dash-", false},
		{strings.Repeat("a", 64), false},
		{"vm/slash", false},
	}
	for _, c := range cases {
		err := ValidateName(c.in)
		ok := err == nil
		if ok != c.want {
			t.Errorf("ValidateName(%q) ok=%v, want %v (err=%v)", c.in, ok, c.want, err)
		}
	}
}

func TestValidateSSHAlias(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"vm-a", true},
		{"vm-a.example.org", true},
		{"user@host", true},
		{"", false},
		{"-evil", false}, // flag smuggling
		{"with space", false},
	}
	for _, c := range cases {
		err := ValidateSSHAlias(c.in)
		ok := err == nil
		if ok != c.want {
			t.Errorf("ValidateSSHAlias(%q) ok=%v, want %v (err=%v)", c.in, ok, c.want, err)
		}
	}
}
