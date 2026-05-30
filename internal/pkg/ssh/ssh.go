// Package ssh wraps the system ssh and scp binaries via os/exec.
// All authentication goes through the user's ssh-agent — this
// package never reads private keys.
package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Result captures the outcome of a remote command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner abstracts process execution so callers can inject fakes
// in tests. The real implementation shells out via os/exec.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
	RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (Result, error)
}

// NewRunner returns the default os/exec-backed Runner.
func NewRunner() Runner { return &realRunner{} }

type realRunner struct{}

func (r *realRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	res := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: cmd.ProcessState.ExitCode(),
	}
	return res, err
}

func (r *realRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: cmd.ProcessState.ExitCode(),
	}, err
}

// mustNotBeFlag returns an error if value would be parsed as an SSH/SCP
// option flag (starts with "-"). Prevents argv flag smuggling when
// operator-supplied strings are placed in positional argv slots.
func mustNotBeFlag(field, value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("%s %q: cannot start with '-' (flag-smuggling guard)", field, value)
	}
	return nil
}

// ErrRemoteFailure is returned by Run when the system ssh binary
// exits with a non-zero code. Useful for callers that want to
// distinguish remote failure from local/transport errors.
var ErrRemoteFailure = errors.New("ssh: remote command failed")
