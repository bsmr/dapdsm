package command

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/gamedb"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestPlayerNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := playerCmd(context.Background(), nil, []string{}, &stdout, &stderr); err == nil {
		t.Error("player no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestPlayerMissingSubVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Only host provided, no sub-verb.
	if err := playerCmd(context.Background(), nil, []string{"vm-a"}, &stdout, &stderr); err == nil {
		t.Error("player host only: err=nil, want non-nil")
	}
}

func TestPlayerUnknownSubVerb(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	if err := playerCmd(context.Background(), c, []string{"vm-a", "frobnicate"}, &stdout, &stderr); err == nil {
		t.Error("player unknown sub: err=nil, want non-nil")
	}
}

func TestPlayerPosRequiresFLSID(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := &core.Core{Store: st, SSH: ssh.NewClient()}
	var stdout, stderr bytes.Buffer
	if err := playerCmd(context.Background(), c, []string{"vm-a", "pos"}, &stdout, &stderr); err == nil {
		t.Error("player pos no fls-id: err=nil, want non-nil")
	}
}

func TestPlayerRegisteredInDispatchTable(t *testing.T) {
	if !Known("player") {
		t.Error(`"player" verb not registered in dispatch table`)
	}
}

func TestPlayerInspectUsage(t *testing.T) {
	var out, errb bytes.Buffer
	err := Dispatch(context.Background(), &core.Core{}, []string{"player", "h", "inspect"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("inspect without fls should be ErrUsage, got %v", err)
	}
}

func TestPlayerSpecHasInspect(t *testing.T) {
	s, ok := SpecFor("player")
	if !ok {
		t.Fatal("no player spec")
	}
	found := false
	for _, o := range s.Args[1].options {
		if o == "inspect" {
			found = true
		}
	}
	if !found {
		t.Fatalf("player spec sub-verbs missing inspect: %v", s.Args[1].options)
	}
}

// searchSeqRunner: Run answers the DB-deployment discovery; RunWithStdin returns
// canned responses in order (PlayerSearch: #1 level-column probe, #2 the search).
type searchSeqRunner struct {
	resp []string
	n    int
}

func (r *searchSeqRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: "funcom-x dunedb 15432 postgres", ExitCode: 0}, nil
}
func (r *searchSeqRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	out := ""
	if r.n < len(r.resp) {
		out = r.resp[r.n]
	}
	r.n++
	return ssh.Result{Stdout: out, ExitCode: 0}, nil
}

func TestPlayerSearchNoQueryListsAll(t *testing.T) {
	rr := &searchSeqRunner{resp: []string{
		"account_id\ncharacter_name\nplayer_pawn_id\n",      // PlayerSearch level-column probe
		"A1|Stilgar|Offline|t1||1\nA2|Chani|Online|t2||1\n", // all players
	}}
	c := &core.Core{SSH: &ssh.Client{Runner: rr}, Store: openTestStore(t)}
	var out, errb bytes.Buffer
	err := Dispatch(context.Background(), c, []string{"player", "h", "search"}, &out, &errb)
	if err != nil {
		t.Fatalf("search with no query should succeed, got %v (%s)", err, errb.String())
	}
	if !strings.Contains(out.String(), "Stilgar") || !strings.Contains(out.String(), "Chani") {
		t.Fatalf("expected all players listed:\n%s", out.String())
	}
}

func TestPrintInspectRendersNewSections(t *testing.T) {
	var b bytes.Buffer
	d := &gamedb.PlayerDetail{
		FLSID: "FLS1", Found: true, CharacterName: "Stilgar", OnlineStatus: "Offline",
		Progression: &gamedb.Progression{TotalSkillPoints: 42, UnspentSkillPoints: 7, TotalXPEarned: 12345, ActivePerks: 2},
		Vitals:      &gamedb.Vitals{CurrentHealth: 150},
		Spice:       &gamedb.SpiceState{SystemStatus: "AddictionDisabled", SpiceVision: "FullyEnabled"},
	}
	printInspect(&b, d)
	out := b.String()
	for _, want := range []string{"skill points", "42", "unspent", "7", "health", "150", "spice", "AddictionDisabled"} {
		if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
			t.Errorf("inspect output missing %q:\n%s", want, out)
		}
	}
}
