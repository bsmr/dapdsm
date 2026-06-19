package crypto

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// fakeRunner records calls and returns canned stdout.
type fakeRunner struct {
	calls  []string
	stdins [][]byte
	stdout string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (ssh.Result, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return ssh.Result{Stdout: f.stdout}, nil
}
func (f *fakeRunner) RunWithStdin(_ context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	f.stdins = append(f.stdins, stdin)
	return ssh.Result{Stdout: f.stdout}, nil
}

func clientWith(f *fakeRunner) *ssh.Client { return &ssh.Client{Runner: f} }

func TestSealPipesPlaintextToAgeRecipient(t *testing.T) {
	f := &fakeRunner{stdout: "SEALED"}
	out, err := Seal(context.Background(), f, "age1abc", []byte("super-secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if string(out) != "SEALED" {
		t.Fatalf("sealed = %q", out)
	}
	joined := strings.Join(f.calls, " | ")
	if !strings.Contains(joined, "age") || !strings.Contains(joined, "-r") || !strings.Contains(joined, "age1abc") {
		t.Fatalf("argv missing age -r recipient: %s", joined)
	}
	if len(f.stdins) != 1 || string(f.stdins[0]) != "super-secret" {
		t.Fatalf("plaintext not piped via stdin: %v", f.stdins)
	}
}

func TestEnsureInstalledChecksAndInstallsAge(t *testing.T) {
	f := &fakeRunner{}
	if err := EnsureInstalled(context.Background(), clientWith(f), "vm-a"); err != nil {
		t.Fatalf("EnsureInstalled: %v", err)
	}
	joined := strings.Join(f.calls, " ")
	if !strings.Contains(joined, "command -v age") || !strings.Contains(joined, "install -y age") {
		t.Fatalf("argv missing age install guard: %s", joined)
	}
}

func TestEnsureIdentityCreatesIfAbsentAndReturnsRecipient(t *testing.T) {
	f := &fakeRunner{stdout: "age1recipientxyz\n"}
	rec, err := EnsureIdentity(context.Background(), clientWith(f), "vm-a", "/etc/dune/age.key")
	if err != nil {
		t.Fatalf("EnsureIdentity: %v", err)
	}
	if rec != "age1recipientxyz" {
		t.Fatalf("recipient = %q", rec)
	}
	joined := strings.Join(f.calls, " ")
	if !strings.Contains(joined, "/etc/dune/age.key") || !strings.Contains(joined, "age-keygen") {
		t.Fatalf("argv missing keygen for keypath: %s", joined)
	}
}

func TestUnsealToFileDecryptsOnVM(t *testing.T) {
	f := &fakeRunner{}
	err := UnsealToFile(context.Background(), clientWith(f), "vm-a",
		"/etc/dune/age.key", "/tmp/fls-token.age", "/etc/dune/fls-token", 0o600)
	if err != nil {
		t.Fatalf("UnsealToFile: %v", err)
	}
	joined := strings.Join(f.calls, " ")
	for _, want := range []string{"age -d", "-i /etc/dune/age.key", "/tmp/fls-token.age", "/etc/dune/fls-token"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv missing %q: %s", want, joined)
		}
	}
}
