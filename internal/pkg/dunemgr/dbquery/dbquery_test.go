package dbquery

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// fakeDBDeploy is the canned `kubectl get databasedeployment` jsonpath
// output discoverDB parses: "<namespace> <name> <port> <superUser>".
const fakeDBDeploy = "funcom-seabass-x funcom-dunedb 15432 postgres"

type fakeRunner struct {
	stdoutMap map[string]string
	calls     []string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	// After the shell-quoting fix ssh.Client passes args as:
	//   ["-o", "BatchMode=yes", "--", host, "<quoted remote cmd>"]
	// The last element contains all remote words individually quoted.
	// Build the joined string and also match against the raw remote token
	// so pattern keys like "get databasedeployment" still work.
	joined := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, joined)
	for k, v := range f.stdoutMap {
		if strings.Contains(joined, k) {
			return ssh.Result{Stdout: v, ExitCode: 0}, nil
		}
		// Also match when each word in the key is individually shell-quoted
		// (e.g. "get databasedeployment" → "'get' 'databasedeployment'").
		quoted := quotedPattern(k)
		if strings.Contains(joined, quoted) {
			return ssh.Result{Stdout: v, ExitCode: 0}, nil
		}
	}
	return ssh.Result{ExitCode: 0}, nil
}

// quotedPattern converts a space-separated key into the shell-quoted form
// produced by ssh.Client after the shell-quoting fix, e.g.
// "get databasedeployment" → "'get' 'databasedeployment'".
func quotedPattern(key string) string {
	words := strings.Fields(key)
	for i, w := range words {
		words[i] = "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
	}
	return strings.Join(words, " ")
}

func (f *fakeRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func TestDiscoverDBHappyPath(t *testing.T) {
	rr := &fakeRunner{stdoutMap: map[string]string{
		"get databasedeployment": fakeDBDeploy + "\n",
	}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}}
	tgt, err := r.discoverDB(context.Background(), "vm-a")
	if err != nil {
		t.Fatalf("discoverDB: %v", err)
	}
	if tgt.Pod != "funcom-dunedb-sts-0" || tgt.Namespace != "funcom-seabass-x" ||
		tgt.Port != 15432 || tgt.SuperUser != "postgres" {
		t.Errorf("target=%+v", tgt)
	}
}

func TestDiscoverDBMissingDeploymentErrors(t *testing.T) {
	rr := &fakeRunner{stdoutMap: map[string]string{"get databasedeployment": ""}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}}
	if _, err := r.discoverDB(context.Background(), "vm-a"); err == nil {
		t.Error("discoverDB empty stdout: err=nil, want non-nil")
	}
}

func TestDiscoverDBRejectsFlagPrefixedName(t *testing.T) {
	rr := &fakeRunner{stdoutMap: map[string]string{
		"get databasedeployment": "funcom-seabass-x -oProxyCommand 15432 postgres\n",
	}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}}
	if _, err := r.discoverDB(context.Background(), "vm-a"); err == nil {
		t.Error("discoverDB flag-prefixed name: err=nil, want non-nil")
	}
}

func TestDiscoverDBBadPortErrors(t *testing.T) {
	rr := &fakeRunner{stdoutMap: map[string]string{
		"get databasedeployment": "funcom-seabass-x funcom-dunedb notaport postgres\n",
	}}
	r := &Runner{SSH: &ssh.Client{Runner: rr}}
	if _, err := r.discoverDB(context.Background(), "vm-a"); err == nil {
		t.Error("discoverDB non-numeric port: err=nil, want non-nil")
	}
}

// exitCodeRunner returns a non-zero kubectl exit with stderr but a nil
// Go error, exercising the exit-code branch of discoverDB.
type exitCodeRunner struct{}

func (exitCodeRunner) Run(context.Context, string, ...string) (ssh.Result, error) {
	return ssh.Result{ExitCode: 1, Stderr: "boom"}, nil
}

func (exitCodeRunner) RunWithStdin(context.Context, []byte, string, ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func TestDiscoverDBNonZeroExitErrors(t *testing.T) {
	r := &Runner{SSH: &ssh.Client{Runner: exitCodeRunner{}}}
	_, err := r.discoverDB(context.Background(), "vm-a")
	if err == nil {
		t.Fatal("discoverDB non-zero exit: err=nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "boom") || strings.Contains(err.Error(), "%!w") {
		t.Errorf("error message malformed: %v", err)
	}
}
