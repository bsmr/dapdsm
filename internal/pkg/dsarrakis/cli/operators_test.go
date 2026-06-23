package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// opExecer answers cat (version.txt) and records kubectl runs/stdin.
// It also returns worker node names for kubectl get nodes calls.
type opExecer struct {
	stdin [][]byte
	runs  [][]string
}

func (e *opExecer) Run(_ context.Context, host, cmd string, args ...string) (ssh.Result, error) {
	e.runs = append(e.runs, append([]string{cmd}, args...))
	if cmd == "cat" {
		if strings.Contains(args[len(args)-1], "version.txt") {
			return ssh.Result{Stdout: "v1.5.0\n"}, nil
		}
		return ssh.Result{Stdout: ""}, nil
	}
	// CRD dir preflight: `test -d <dir>` — dir present by default.
	if cmd == "test" {
		return ssh.Result{}, nil
	}
	// kubectl get nodes: return worker node names so workerNodes() succeeds.
	if cmd == "env" && containsArg(args, "get") && containsArg(args, "nodes") {
		return ssh.Result{Stdout: "w1 w2"}, nil
	}
	// `get secret` must fail so the webhook-secret apply path runs
	if cmd == "env" && containsArg(args, "get") {
		return ssh.Result{}, &exitErr{}
	}
	return ssh.Result{}, nil
}
func (e *opExecer) RunWithStdin(_ context.Context, host string, in []byte, cmd string, args ...string) (ssh.Result, error) {
	e.stdin = append(e.stdin, in)
	return ssh.Result{}, nil
}

type exitErr struct{}

func (*exitErr) Error() string { return "exit status 1" }

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestOperatorsCmd_Bringup(t *testing.T) {
	e := &opExecer{}
	var out, errOut bytes.Buffer
	err := operatorsCmd(context.Background(), e, []string{
		"bringup", "--jump", "j", "--kubeconfig", "/kc",
		"--env", "prod",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("operatorsCmd: %v\nstderr: %s", err, errOut.String())
	}
	// operator manifest applied via stdin (rendered with the read version)
	sawOperators := false
	for _, m := range e.stdin {
		if strings.Contains(string(m), "battlegroupoperator-controller-manager") &&
			strings.Contains(string(m), "registry.funcom.com/funcom/self-hosting/igw-k8s-battlegroup-operator:v1.5.0") {
			sawOperators = true
		}
	}
	if !sawOperators {
		t.Error("operator manifest not applied via stdin with original registry.funcom.com ref")
	}
}

func TestOperatorsCmd_MissingFlags(t *testing.T) {
	e := &opExecer{}
	var out, errOut bytes.Buffer
	if err := operatorsCmd(context.Background(), e, []string{"bringup", "--jump", "j"}, &out, &errOut); err == nil {
		t.Fatal("expected error when required flags missing")
	}
}

// missingCRDExecer behaves like opExecer but returns an error for `test -d`.
type missingCRDExecer struct{ opExecer }

func (e *missingCRDExecer) Run(ctx context.Context, host, cmd string, args ...string) (ssh.Result, error) {
	if cmd == "test" {
		return ssh.Result{}, &exitErr{}
	}
	return e.opExecer.Run(ctx, host, cmd, args...)
}

func (e *missingCRDExecer) RunWithStdin(ctx context.Context, host string, in []byte, cmd string, args ...string) (ssh.Result, error) {
	return e.opExecer.RunWithStdin(ctx, host, in, cmd, args...)
}

var _ clusteraccess.Execer = (*missingCRDExecer)(nil)

func TestOperatorsCmd_MissingCRDDir(t *testing.T) {
	e := &missingCRDExecer{}
	var out, errOut bytes.Buffer
	err := operatorsCmd(context.Background(), e, []string{
		"bringup", "--jump", "j", "--kubeconfig", "/kc",
		"--env", "prod",
	}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error when CRD dir is absent")
	}
	if !strings.Contains(err.Error(), "operator CRD dir not found on jumphost") {
		t.Errorf("unexpected error message: %v", err)
	}
}

var _ clusteraccess.Execer = (*opExecer)(nil)
