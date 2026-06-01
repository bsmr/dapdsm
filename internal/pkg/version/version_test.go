package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestString_AlwaysContainsVersionAndRuntime(t *testing.T) {
	t.Parallel()
	s := String("dunectl")
	if !strings.Contains(s, "dunectl ") {
		t.Errorf("missing 'dunectl ' prefix: %q", s)
	}
	if !strings.Contains(s, Version) {
		t.Errorf("missing Version %q: %q", Version, s)
	}
	if !strings.Contains(s, runtime.Version()) {
		t.Errorf("missing Go runtime %q: %q", runtime.Version(), s)
	}
	if !strings.Contains(s, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("missing GOOS/GOARCH %q/%q: %q", runtime.GOOS, runtime.GOARCH, s)
	}
}

func TestString_UsesGivenToolName(t *testing.T) {
	t.Parallel()
	s := String("dunemgr")
	if !strings.HasPrefix(s, "dunemgr "+Version) {
		t.Errorf("want prefix %q, got %q", "dunemgr "+Version, s)
	}
	if strings.Contains(s, "dunectl") {
		t.Errorf("dunemgr build line must not mention dunectl: %q", s)
	}
}
