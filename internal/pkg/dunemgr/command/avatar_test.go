package command

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAvatarKnown(t *testing.T) {
	if !Known("avatar") {
		t.Fatal("avatar should be a known verb")
	}
}

func TestAvatarUsageErrors(t *testing.T) {
	cases := [][]string{
		{"avatar"},                     // no sub
		{"avatar", "bogus"},            // unknown sub
		{"avatar", "export"},           // export missing fls
		{"avatar", "import", "h", "f"}, // import missing key
		{"avatar", "transfer", "a"},    // transfer missing dst+fls
	}
	for _, argv := range cases {
		var out, errb bytes.Buffer
		err := Dispatch(context.Background(), &core.Core{}, argv, &out, &errb)
		if !errors.Is(err, ErrUsage) {
			t.Fatalf("argv %v: want ErrUsage, got %v", argv, err)
		}
	}
}

func TestAvatarUnknownPlayerIsUsageError(t *testing.T) {
	var out, errb bytes.Buffer
	// SSH present so discoverDB runs and fails (no such host) → resolvePlayerArg wraps as ErrUsage.
	c := &core.Core{Store: openTestStore(t), SSH: ssh.NewClient()}
	err := avatarCmd(context.Background(), c, []string{"export", "h", "NoSuchName"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("unknown name should be ErrUsage, got %v", err)
	}
}

func TestAvatarImportRefusesWithoutConfirm(t *testing.T) {
	var out, errb bytes.Buffer
	// --id bypasses name resolution (SSH nil) so the --confirm gate is what fails.
	c := &core.Core{Store: openTestStore(t), SSH: ssh.NewClient()}
	err := avatarCmd(context.Background(), c, []string{"import", "h", "fls", "key", "--id"}, &out, &errb)
	if err == nil {
		t.Fatal("import without --confirm must error")
	}
}
