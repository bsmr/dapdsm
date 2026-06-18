package db

import (
	"context"
	"reflect"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

type recSSH struct {
	gotHost, gotName string
	gotStdin         []byte
	gotArgs          []string
	res              ssh.Result
}

func (r *recSSH) Run(_ context.Context, host, name string, args ...string) (ssh.Result, error) {
	r.gotHost, r.gotName, r.gotArgs = host, name, args
	return r.res, nil
}

func (r *recSSH) RunWithStdin(_ context.Context, host string, stdin []byte, name string, args ...string) (ssh.Result, error) {
	r.gotHost, r.gotStdin, r.gotName, r.gotArgs = host, stdin, name, args
	return r.res, nil
}

func TestSSHExecerWrapsExecWithStdin(t *testing.T) {
	rr := &recSSH{res: ssh.Result{Stdout: "ok\n"}}
	e := sshExecer{c: rr, host: "vm1"}
	out, err := e.Run(context.Background(), "ns", "pod0", []byte("SELECT 1;"), "psql", "-tA")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok\n" || rr.gotHost != "vm1" || rr.gotName != "kubectl" || string(rr.gotStdin) != "SELECT 1;" {
		t.Fatalf("host=%q name=%q stdin=%q out=%q", rr.gotHost, rr.gotName, rr.gotStdin, out)
	}
	want := []string{"exec", "-i", "-n", "ns", "pod0", "--", "psql", "-tA"}
	if !reflect.DeepEqual(rr.gotArgs, want) {
		t.Fatalf("args = %v want %v", rr.gotArgs, want)
	}
}

func TestSSHExecerNoStdinOmitsDashI(t *testing.T) {
	rr := &recSSH{res: ssh.Result{Stdout: "x"}}
	e := sshExecer{c: rr, host: "vm1"}
	if _, err := e.Run(context.Background(), "ns", "pod0", nil, "psql", "-c", "SELECT 1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec", "-n", "ns", "pod0", "--", "psql", "-c", "SELECT 1"}
	if !reflect.DeepEqual(rr.gotArgs, want) {
		t.Fatalf("args = %v want %v", rr.gotArgs, want)
	}
}

func TestSSHExecerNonZeroExitIsError(t *testing.T) {
	rr := &recSSH{res: ssh.Result{ExitCode: 1, Stderr: "boom"}}
	e := sshExecer{c: rr, host: "vm1"}
	if _, err := e.Run(context.Background(), "ns", "pod0", []byte("x"), "psql"); err == nil {
		t.Fatal("want error on non-zero exit")
	}
}
