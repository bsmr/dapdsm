// Package kubedist installs and readies a Kubernetes distribution (k3s,
// rke2) on the local node. The Runner here is a local command executor;
// remote execution is the caller's concern (pkg/transport/ssh).
package kubedist

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

func NewRunner() Runner { return &execRunner{} }

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %w: %s", name, err, stderr.String())
	}
	return stdout.String(), nil
}

// FakeRunner is a test double shared across kubedist/steamcmd tests.
type FakeRunner struct {
	Calls   [][]string
	Outputs map[string]string // keyed by command name
	Errs    map[string]error
}

func (f *FakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.Calls = append(f.Calls, append([]string{name}, args...))
	return f.Outputs[name], f.Errs[name]
}
