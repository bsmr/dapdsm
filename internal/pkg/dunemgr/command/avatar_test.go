package command

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
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
		{"avatar"},                       // no host
		{"avatar", "h"},                  // no sub
		{"avatar", "h", "bogus"},         // unknown sub
		{"avatar", "h", "export"},        // export missing fls
		{"avatar", "h", "import", "f"},   // import missing key
		{"avatar", "h", "transfer", "a"}, // transfer missing dst+fls
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
	err := avatarCmd(context.Background(), c, []string{"h", "export", "NoSuchName"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("unknown name should be ErrUsage, got %v", err)
	}
}

func TestAvatarImportRefusesWithoutConfirm(t *testing.T) {
	var out, errb bytes.Buffer
	// --id bypasses name resolution (SSH nil) so the --confirm gate is what fails.
	c := &core.Core{Store: openTestStore(t), SSH: ssh.NewClient()}
	err := avatarCmd(context.Background(), c, []string{"h", "import", "fls", "key", "--id"}, &out, &errb)
	if err == nil {
		t.Fatal("import without --confirm must error")
	}
}

// TestAvatarHostFirstUsage verifies the new host-first grammar guard:
// providing only one arg (the sub-verb, no host) must return ErrUsage
// and print the host-first usage hint. c is nil to confirm no nil-panic.
func TestAvatarHostFirstUsage(t *testing.T) {
	var out, errb bytes.Buffer
	err := avatarCmd(context.Background(), nil, []string{"list"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("avatar list (missing host) err=%v, want ErrUsage", err)
	}
	if !strings.Contains(errb.String(), "avatar <host>") {
		t.Errorf("usage should show host-first grammar, got %q", errb.String())
	}
}

// TestAvatarListServerOutputsPlayers verifies that avatar list queries
// PlayerSearch and renders all returned character names to stdout.
// The fake uses the same searchSeqRunner pattern from player_test.go.
func TestAvatarListServerOutputsPlayers(t *testing.T) {
	rr := &searchSeqRunner{resp: []string{
		"account_id\ncharacter_name\nplayer_pawn_id\n",                                        // level-column probe
		"A1|Stilgar|Offline|2024-01-01 00:00:00||1\nA2|Chani|Online|2024-01-02 00:00:00||1\n", // search result
	}}
	c := &core.Core{SSH: &ssh.Client{Runner: rr}, Store: openTestStore(t)}
	var out, errb bytes.Buffer
	err := avatarCmd(context.Background(), c, []string{"vm-a", "list"}, &out, &errb)
	if err != nil {
		t.Fatalf("avatar list err=%v (%s)", err, errb.String())
	}
	if !strings.Contains(out.String(), "Stilgar") || !strings.Contains(out.String(), "Chani") {
		t.Fatalf("avatar list output missing expected names:\n%s", out.String())
	}
}

// TestAvatarListAlignedNoTabs guards the listing alignment: avatar list must
// emit space-aligned columns (via tabwriter), never raw tabs, so the FLS column
// lines up regardless of character-name length. Line 0 is the header; data
// rows (Stilgar=A1, Chani=A2) are on lines[1] and lines[2].
func TestAvatarListAlignedNoTabs(t *testing.T) {
	rr := &searchSeqRunner{resp: []string{
		"account_id\ncharacter_name\nplayer_pawn_id\n",
		"A1|Stilgar|Offline|2024-01-01 00:00:00||1\nA2|Chani|Online|2024-01-02 00:00:00||1\n",
	}}
	c := &core.Core{SSH: &ssh.Client{Runner: rr}, Store: openTestStore(t)}
	var out, errb bytes.Buffer
	if err := avatarCmd(context.Background(), c, []string{"vm-a", "list"}, &out, &errb); err != nil {
		t.Fatalf("avatar list err=%v (%s)", err, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "\t") {
		t.Fatalf("avatar list must not emit raw tabs:\n%q", s)
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	// lines[0] = header; lines[1] = Stilgar (A1); lines[2] = Chani (A2)
	if len(lines) < 3 {
		t.Fatalf("want header + >=2 data rows, got %d lines:\n%s", len(lines), s)
	}
	// Find the data lines by FLS-ID content, then compare column offsets.
	var stilgarLine, chaniLine string
	for _, l := range lines[1:] {
		if strings.Contains(l, "A1") {
			stilgarLine = l
		}
		if strings.Contains(l, "A2") {
			chaniLine = l
		}
	}
	if stilgarLine == "" || chaniLine == "" {
		t.Fatalf("could not locate A1/A2 data lines:\n%s", s)
	}
	if a, b := strings.Index(stilgarLine, "A1"), strings.Index(chaniLine, "A2"); a != b {
		t.Fatalf("FLS column misaligned: %d vs %d\n%q\n%q", a, b, stilgarLine, chaniLine)
	}
}

// TestAvatarListHasHeaderRow verifies that avatar list emits a header line
// (containing NAME and FLS-ID) as the first line of output.
func TestAvatarListHasHeaderRow(t *testing.T) {
	rr := &searchSeqRunner{resp: []string{
		"account_id\ncharacter_name\nplayer_pawn_id\n",
		"A1|Stilgar|Offline|2024-01-01 00:00:00||1\n",
	}}
	c := &core.Core{SSH: &ssh.Client{Runner: rr}, Store: openTestStore(t)}
	var out, errb bytes.Buffer
	if err := avatarCmd(context.Background(), c, []string{"vm-a", "list"}, &out, &errb); err != nil {
		t.Fatalf("avatar list err=%v (%s)", err, errb.String())
	}
	first := strings.SplitN(out.String(), "\n", 2)[0]
	if !strings.Contains(first, "NAME") || !strings.Contains(first, "FLS-ID") {
		t.Fatalf("first line should be a header, got %q", first)
	}
}

// TestAvatarExportsAlignedNoTabs guards the same alignment for the local
// exports listing.
func TestAvatarExportsAlignedNoTabs(t *testing.T) {
	s := openTestStore(t)
	if err := s.PutExport(store.ExportRecord{Host: "vm-a", FLSID: "DEADBEEF", UnixTS: 1, CharacterName: "Stilgar", Bytes: 10}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutExport(store.ExportRecord{Host: "vm-a", FLSID: "C0FFEE01", UnixTS: 2, CharacterName: "Chani", Bytes: 20}); err != nil {
		t.Fatal(err)
	}
	c := &core.Core{Store: s}
	var out, errb bytes.Buffer
	if err := avatarCmd(context.Background(), c, []string{"vm-a", "exports"}, &out, &errb); err != nil {
		t.Fatalf("avatar exports err=%v (%s)", err, errb.String())
	}
	if strings.Contains(out.String(), "\t") {
		t.Fatalf("avatar exports must not emit raw tabs:\n%q", out.String())
	}
}
