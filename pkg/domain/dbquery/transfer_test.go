package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// xferRunner is a fake ssh.Runner: Run answers discoverDB with fakeDBDeploy;
// RunWithStdin records the piped SQL + argv and returns a canned stdout.
type xferRunner struct {
	stdinSQL string
	args     []string
	reply    string
}

func (r *xferRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: fakeDBDeploy, ExitCode: 0}, nil
}

func (r *xferRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	r.stdinSQL = string(stdin)
	r.args = args
	return ssh.Result{Stdout: r.reply, ExitCode: 0}, nil
}

func newXferRunner(t *testing.T, reply string) (*Runner, *xferRunner) {
	t.Helper()
	rr := &xferRunner{reply: reply}
	return &Runner{SSH: &ssh.Client{Runner: rr}, Store: newTempStore(t)}, rr
}

// hasVar reports whether a -v kv pair exists in args. It handles both the
// raw form (two adjacent args: "-v", "kv") and the shell-quoted single-token
// form produced by ssh.Client ("'-v' 'kv'" embedded in the last arg).
func hasVar(args []string, kv string) bool {
	// Raw form: adjacent "-v" and kv elements.
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-v" && args[i+1] == kv {
			return true
		}
	}
	// Shell-quoted form: "'-v' '<kv>'" within a single remote-command token.
	quoted := "'-v' '" + strings.ReplaceAll(kv, "'", `'\''`) + "'"
	joined := strings.Join(args, " ")
	return strings.Contains(joined, quoted)
}

// hasPGOptions reports whether PGOPTIONS is injected into the recorded argv.
// It accepts both the raw two-element form ("env", "PGOPTIONS=...") and the
// shell-quoted single-token form produced by ssh.Client.
func hasPGOptions(args []string) bool {
	val := "PGOPTIONS=-c search_path=dune,public"
	// Raw form: "env" followed immediately by the value.
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "env" && args[i+1] == val {
			return true
		}
	}
	// Shell-quoted form embedded in the remote-command token.
	quoted := "'env' '" + val + "'"
	joined := strings.Join(args, " ")
	return strings.Contains(joined, quoted)
}

func TestIsPlayerOffline(t *testing.T) {
	r, rr := newXferRunner(t, "t\n")
	off, err := r.IsPlayerOffline(context.Background(), "h", "fls-1")
	if err != nil {
		t.Fatal(err)
	}
	if !off {
		t.Fatal("want offline=true for 't'")
	}
	if !strings.Contains(rr.stdinSQL, "is_player_offline") {
		t.Fatalf("SQL missing is_player_offline: %q", rr.stdinSQL)
	}
	if !hasVar(rr.args, "fls=fls-1") {
		t.Fatalf("fls not bound as -v: %v", rr.args)
	}
	if strings.Contains(rr.stdinSQL, "fls-1") {
		t.Fatal("fls value leaked into SQL body (injection risk)")
	}
	// Fix: PGOPTIONS must inject search_path so Funcom unqualified refs resolve.
	if !hasPGOptions(rr.args) {
		t.Fatalf("PGOPTIONS not injected in argv: %v", rr.args)
	}
	// Fix: ON_ERROR_STOP must be prepended so DB RAISEs surface as errors.
	if !strings.HasPrefix(rr.stdinSQL, `\set ON_ERROR_STOP on`) {
		t.Fatalf("stdinSQL missing ON_ERROR_STOP prefix: %q", rr.stdinSQL)
	}
}

func TestPatchesChecksum(t *testing.T) {
	r, rr := newXferRunner(t, "abc123checksum\n")
	got, err := r.PatchesChecksum(context.Background(), "h")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc123checksum" {
		t.Fatalf("want abc123checksum, got %q", got)
	}
	// Fix: PGOPTIONS + ON_ERROR_STOP.
	if !hasPGOptions(rr.args) {
		t.Fatalf("PGOPTIONS not injected in argv: %v", rr.args)
	}
	if !strings.HasPrefix(rr.stdinSQL, `\set ON_ERROR_STOP on`) {
		t.Fatalf("stdinSQL missing ON_ERROR_STOP prefix: %q", rr.stdinSQL)
	}
}

func TestCharacterExport(t *testing.T) {
	r, rr := newXferRunner(t, `{"_patches_checksum":"cs","funcom_id":"fls-1"}`+"\n")
	out, err := r.CharacterExport(context.Background(), "h", "fls-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "_patches_checksum") {
		t.Fatalf("export not returned: %q", out)
	}
	if !strings.Contains(rr.stdinSQL, "character_transfer_export") {
		t.Fatalf("SQL missing export call: %q", rr.stdinSQL)
	}
	// Fix: PGOPTIONS + ON_ERROR_STOP.
	if !hasPGOptions(rr.args) {
		t.Fatalf("PGOPTIONS not injected in argv: %v", rr.args)
	}
	if !strings.HasPrefix(rr.stdinSQL, `\set ON_ERROR_STOP on`) {
		t.Fatalf("stdinSQL missing ON_ERROR_STOP prefix: %q", rr.stdinSQL)
	}
}

func TestCharacterImportBase64AndVars(t *testing.T) {
	r, rr := newXferRunner(t, "987654\n")
	id, err := r.CharacterImport(context.Background(), "h", `{"a":1}`, "fls-1", "Paul")
	if err != nil {
		t.Fatal(err)
	}
	if id != 987654 {
		t.Fatalf("want controller id 987654, got %d", id)
	}
	if !strings.Contains(rr.stdinSQL, "$b64$") || !strings.Contains(rr.stdinSQL, "decode(") {
		t.Fatalf("import SQL missing base64 dollar-quote: %q", rr.stdinSQL)
	}
	if hasVar(rr.args, `data={"a":1}`) {
		t.Fatal("blob must not be a -v argv value (ARG_MAX risk)")
	}
	if !hasVar(rr.args, "fls=fls-1") || !hasVar(rr.args, "name=Paul") {
		t.Fatalf("fls/name not bound as -v: %v", rr.args)
	}
	if strings.Contains(rr.stdinSQL, `{"a":1}`) {
		t.Fatal("raw JSON must not appear in SQL body (it is base64-encoded)")
	}
	// Fix: PGOPTIONS + ON_ERROR_STOP.
	if !hasPGOptions(rr.args) {
		t.Fatalf("PGOPTIONS not injected in argv: %v", rr.args)
	}
	if !strings.HasPrefix(rr.stdinSQL, `\set ON_ERROR_STOP on`) {
		t.Fatalf("stdinSQL missing ON_ERROR_STOP prefix: %q", rr.stdinSQL)
	}
}

func TestCharacterName(t *testing.T) {
	r, rr := newXferRunner(t, "Chani\n")
	name, err := r.CharacterName(context.Background(), "h", "fls-1")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Chani" {
		t.Fatalf("want Chani, got %q", name)
	}
	if !hasVar(rr.args, "fls=fls-1") {
		t.Fatalf("fls not bound: %v", rr.args)
	}
	// Fix: PGOPTIONS + ON_ERROR_STOP.
	if !hasPGOptions(rr.args) {
		t.Fatalf("PGOPTIONS not injected in argv: %v", rr.args)
	}
	if !strings.HasPrefix(rr.stdinSQL, `\set ON_ERROR_STOP on`) {
		t.Fatalf("stdinSQL missing ON_ERROR_STOP prefix: %q", rr.stdinSQL)
	}
}
