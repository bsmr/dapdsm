package ssh

import "context"

// Client groups SSH operations against remote hosts. The Runner
// field is injectable for testing; nil falls back to NewRunner().
type Client struct {
	Runner Runner
}

// NewClient returns a Client with the default os/exec Runner.
func NewClient() *Client { return &Client{Runner: NewRunner()} }

func (c *Client) runner() Runner {
	if c.Runner == nil {
		return NewRunner()
	}
	return c.Runner
}

// Run executes `ssh -o BatchMode=yes <host> <cmd> <args...>`. The
// host is an alias resolved by the operator's ~/.ssh/config —
// ProxyJump, IdentityFile, certs, FIDO/U2F, etc. are honored by
// the system ssh binary.
func (c *Client) Run(ctx context.Context, host, cmd string, args ...string) (Result, error) {
	if err := mustNotBeFlag("host", host); err != nil {
		return Result{}, err
	}
	full := append([]string{"-o", "BatchMode=yes", "--", host, cmd}, args...)
	return c.runner().Run(ctx, "ssh", full...)
}

// RunWithStdin runs ssh against host with stdin piped to the remote
// command's standard input. Used for tools like
// `kubectl exec -i pod -- sh -lc <shell>` that read their payload
// from stdin.
func (c *Client) RunWithStdin(ctx context.Context, host string, stdin []byte, cmd string, args ...string) (Result, error) {
	if err := mustNotBeFlag("host", host); err != nil {
		return Result{}, err
	}
	full := append([]string{"-o", "BatchMode=yes", "--", host, cmd}, args...)
	return c.runner().RunWithStdin(ctx, stdin, "ssh", full...)
}
