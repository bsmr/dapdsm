package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

type resolveRunner struct {
	resp []string
	n    int
	sqls []string
}

func (r *resolveRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: fakeDBDeploy, ExitCode: 0}, nil
}
func (r *resolveRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	r.sqls = append(r.sqls, string(stdin))
	out := ""
	if r.n < len(r.resp) {
		out = r.resp[r.n]
	}
	r.n++
	return ssh.Result{Stdout: out, ExitCode: 0}, nil
}

const probeCols = "account_id\ncharacter_name\nplayer_pawn_id\n"

func newResolveRunner(searchRows string) *resolveRunner {
	return &resolveRunner{resp: []string{probeCols, searchRows}}
}

func TestResolveFLSShapeShortCircuits(t *testing.T) {
	rr := newResolveRunner("")
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	fls, amb, err := r.ResolvePlayerRef(context.Background(), "h", "127AC6307755DB02")
	if err != nil || amb != nil || fls != "127AC6307755DB02" {
		t.Fatalf("fls shape should pass through: fls=%q amb=%v err=%v", fls, amb, err)
	}
	if len(rr.sqls) != 0 {
		t.Fatalf("fls shape must not hit the DB, got %d queries", len(rr.sqls))
	}
}

func TestResolveExactName(t *testing.T) {
	rr := newResolveRunner("ABCD000000000001|Stilgar|Offline|2026-06-01 10:00:00||1\n")
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	fls, amb, err := r.ResolvePlayerRef(context.Background(), "h", "Stilgar")
	if err != nil || amb != nil || fls != "ABCD000000000001" {
		t.Fatalf("exact: fls=%q amb=%v err=%v", fls, amb, err)
	}
	if !strings.Contains(strings.Join(rr.sqls, ""), "user") {
		t.Fatalf("expected a search query")
	}
}

func TestResolvePrefixUnique(t *testing.T) {
	rr := newResolveRunner("ABCD000000000009|Stilburn|Offline|2026-06-01 10:00:00||1\n")
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	fls, amb, err := r.ResolvePlayerRef(context.Background(), "h", "Stil")
	if err != nil || amb != nil || fls != "ABCD000000000009" {
		t.Fatalf("prefix-unique: fls=%q amb=%v err=%v", fls, amb, err)
	}
}

func TestResolveAmbiguous(t *testing.T) {
	rr := newResolveRunner("A1|Stilgar|Offline|t1||1\nA2|Stilburn|Offline|t2||1\n")
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	fls, amb, err := r.ResolvePlayerRef(context.Background(), "h", "Stil")
	if err != nil || fls != "" || len(amb) != 2 {
		t.Fatalf("ambiguous: fls=%q amb=%v err=%v", fls, amb, err)
	}
}

func TestResolveNone(t *testing.T) {
	rr := newResolveRunner("")
	r := &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}
	_, _, err := r.ResolvePlayerRef(context.Background(), "h", "Nobody")
	if err == nil {
		t.Fatal("no match must be an error")
	}
}
