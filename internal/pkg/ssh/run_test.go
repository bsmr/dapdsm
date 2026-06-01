package ssh

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	gotName string
	gotArgs []string
	out     Result
	err     error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	f.gotName = name
	f.gotArgs = args
	return f.out, f.err
}

func (f *fakeRunner) RunWithStdin(_ context.Context, _ []byte, _ string, _ ...string) (Result, error) {
	return Result{}, nil
}

// TestRunPassesBatchModeAndHost verifies that the local ssh invocation uses
// BatchMode=yes, --, and the host, and that cmd+args are merged into a single
// quoted remote-command token after the host.
func TestRunPassesBatchModeAndHost(t *testing.T) {
	f := &fakeRunner{out: Result{Stdout: "ok\n"}}
	c := &Client{Runner: f}
	res, err := c.Run(context.Background(), "vm-a", "echo", "ok")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stdout != "ok\n" {
		t.Errorf("stdout = %q, want %q", res.Stdout, "ok\n")
	}
	if f.gotName != "ssh" {
		t.Errorf("invoked %q, want ssh", f.gotName)
	}
	// After the fix the argv must be exactly:
	//   ["-o", "BatchMode=yes", "--", "vm-a", "'echo' 'ok'"]
	// — five elements, with the remote command as a single quoted token.
	wantPrefix := []string{"-o", "BatchMode=yes", "--", "vm-a"}
	if len(f.gotArgs) != len(wantPrefix)+1 {
		t.Fatalf("args = %v, want %d elements (prefix + 1 remote token)", f.gotArgs, len(wantPrefix)+1)
	}
	for i, w := range wantPrefix {
		if f.gotArgs[i] != w {
			t.Errorf("arg[%d] = %q, want %q", i, f.gotArgs[i], w)
		}
	}
	remoteArg := f.gotArgs[len(wantPrefix)]
	wantRemote := shellJoin("echo", []string{"ok"})
	if remoteArg != wantRemote {
		t.Errorf("remote arg = %q, want %q", remoteArg, wantRemote)
	}
}

func TestRunPropagatesError(t *testing.T) {
	want := errors.New("boom")
	f := &fakeRunner{err: want}
	c := &Client{Runner: f}
	_, err := c.Run(context.Background(), "vm-a", "false")
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestRunRejectsFlagSmugglingHost(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	_, err := c.Run(context.Background(), "-oProxyCommand=evil", "echo", "x")
	if err == nil {
		t.Errorf("Run with flag-prefixed host: want error, got nil")
	}
}

// TestRunRemoteArgIsSingleToken verifies that a multi-word arg (e.g. a jsonpath
// expression with spaces) ends up as ONE element in the argv passed to the
// runner — it must not be split by the remote shell.
func TestRunRemoteArgIsSingleToken(t *testing.T) {
	f := &fakeRunner{out: Result{}}
	c := &Client{Runner: f}
	_, _ = c.Run(context.Background(), "vm-a", "kubectl", "get", "nodes", "-o", "jsonpath={a} {b} {c}")

	// There must be exactly 5 args to the runner: -o BatchMode=yes -- host <remote>
	if len(f.gotArgs) != 5 {
		t.Fatalf("args len = %d, want 5; args = %v", len(f.gotArgs), f.gotArgs)
	}
	remoteArg := f.gotArgs[4]
	// The jsonpath expression must appear verbatim inside the single remote token.
	if !contains(remoteArg, "jsonpath={a} {b} {c}") {
		t.Errorf("jsonpath not intact in remote arg: %q", remoteArg)
	}
}

// TestShellQuote covers the escaping logic for single-quote-containing strings.
func TestShellQuote(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"it's", `'it'\''s'`},
		{"", "''"},
		{"a'b'c", `'a'\''b'\''c'`},
	}
	for _, tc := range cases {
		got := shellQuote(tc.in)
		if got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestShellJoin covers the full join-and-quote path including metachars.
func TestShellJoin(t *testing.T) {
	cases := []struct {
		cmd  string
		args []string
		want string
	}{
		{"echo", []string{"hello"}, "'echo' 'hello'"},
		{"sh", []string{"-lc", "echo foo; echo bar"}, "'sh' '-lc' 'echo foo; echo bar'"},
		{"kubectl", []string{"get", "nodes", "-o", "jsonpath={a} {b}"}, "'kubectl' 'get' 'nodes' '-o' 'jsonpath={a} {b}'"},
		{"kubectl", []string{"exec", "--", "sh", "-lc", "cmd | grep x"}, "'kubectl' 'exec' '--' 'sh' '-lc' 'cmd | grep x'"},
		{"kubectl", []string{"get", "databasedeployment", "-A", "-o", "jsonpath={a} {b}"}, "'kubectl' 'get' 'databasedeployment' '-A' '-o' 'jsonpath={a} {b}'"},
		// metachars: |, $, ", ;
		{"echo", []string{"$VAR", `"quoted"`, "a|b", "a;b"}, `'echo' '$VAR' '"quoted"' 'a|b' 'a;b'`},
	}
	for _, tc := range cases {
		got := shellJoin(tc.cmd, tc.args)
		if got != tc.want {
			t.Errorf("shellJoin(%q, %v)\n  got  %q\n  want %q", tc.cmd, tc.args, got, tc.want)
		}
	}
}
