package db

import (
	"context"
	"fmt"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// sshRunner is the subset of ssh.Client's public methods used by sshExecer.
// Defined here so tests can inject a fake without going through the ssh binary.
type sshRunner interface {
	Run(ctx context.Context, host, cmd string, args ...string) (ssh.Result, error)
	RunWithStdin(ctx context.Context, host string, stdin []byte, cmd string, args ...string) (ssh.Result, error)
}

// sshExecer runs the in-pod command via SSH+kubectl against a host. Unlike the
// local kube adapter (where kube.Runner builds the exec wrapper), it builds the
// `kubectl exec [-i] -n <ns> <pod> --` wrapper itself.
type sshExecer struct {
	c    sshRunner
	host string
}

// NewSSHExecer adapts an ssh.Client (bound to host) to PodExecer — dunemgr's
// workstation path (kubectl reached over SSH).
func NewSSHExecer(c *ssh.Client, host string) PodExecer { return sshExecer{c: c, host: host} }

func (s sshExecer) Run(ctx context.Context, ns, pod string, stdin []byte, command ...string) (string, error) {
	argv := []string{"exec"}
	if stdin != nil {
		argv = append(argv, "-i")
	}
	argv = append(argv, "-n", ns, pod, "--")
	argv = append(argv, command...)

	var res ssh.Result
	var err error
	if stdin != nil {
		res, err = s.c.RunWithStdin(ctx, s.host, stdin, "kubectl", argv...)
	} else {
		res, err = s.c.Run(ctx, s.host, "kubectl", argv...)
	}
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}
