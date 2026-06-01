package ssh

import (
	"context"
	"strings"
)

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

// shellQuote wraps s in single quotes, escaping embedded single quotes, so the
// remote shell receives it as one literal token.
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// shellJoin renders cmd+args as a single remote-shell command string suitable
// for passing as one argument to the local ssh binary. ssh concatenates all
// post-host arguments with spaces and the remote shell re-splits them; by
// quoting every token here we ensure the remote shell receives each original
// argument intact, even when it contains spaces, shell metacharacters, or
// jsonpath expressions.
func shellJoin(cmd string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, shellQuote(cmd))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

// Run executes `ssh -o BatchMode=yes -- <host> <remote>` where <remote> is
// cmd and args shell-quoted into a single token. The host is an alias resolved
// by the operator's ~/.ssh/config — ProxyJump, IdentityFile, certs,
// FIDO/U2F, etc. are honored by the system ssh binary.
func (c *Client) Run(ctx context.Context, host, cmd string, args ...string) (Result, error) {
	if err := mustNotBeFlag("host", host); err != nil {
		return Result{}, err
	}
	remote := shellJoin(cmd, args)
	full := []string{"-o", "BatchMode=yes", "--", host, remote}
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
	remote := shellJoin(cmd, args)
	full := []string{"-o", "BatchMode=yes", "--", host, remote}
	return c.runner().RunWithStdin(ctx, stdin, "ssh", full...)
}
