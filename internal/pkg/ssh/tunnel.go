package ssh

import (
	"context"
	"fmt"
	"net"
)

// Connect starts an SSH ControlMaster against host, with the
// control socket at sockPath. Runs `ssh -M -S <sock> -N <host>` —
// the master persists until Disconnect is called.
func (c *Client) Connect(ctx context.Context, host, sockPath string) error {
	if err := mustNotBeFlag("host", host); err != nil {
		return err
	}
	_, err := c.runner().Run(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-M", "-S", sockPath, "-N", "--", host,
	)
	return err
}

// OpenTunnel asks an established ControlMaster (identified by
// sockPath) to add a -L forward from 127.0.0.1:localPort to
// targetHost:targetPort. No new SSH handshake — the existing
// connection is reused. host must match the alias the master was
// started with (Connect).
func (c *Client) OpenTunnel(ctx context.Context, host, sockPath string, localPort int, targetHost string, targetPort int) error {
	if err := mustNotBeFlag("host", host); err != nil {
		return err
	}
	if err := mustNotBeFlag("targetHost", targetHost); err != nil {
		return err
	}
	forward := fmt.Sprintf("127.0.0.1:%d:%s:%d", localPort, targetHost, targetPort)
	_, err := c.runner().Run(ctx, "ssh",
		"-S", sockPath,
		"-O", "forward",
		"-L", forward,
		"--", host,
	)
	return err
}

// Disconnect closes the ControlMaster identified by sockPath. All
// active tunnels on that master go down with it.
func (c *Client) Disconnect(ctx context.Context, sockPath, host string) error {
	if err := mustNotBeFlag("host", host); err != nil {
		return err
	}
	_, err := c.runner().Run(ctx, "ssh",
		"-S", sockPath, "-O", "exit", "--", host,
	)
	return err
}

// AllocPort binds to 127.0.0.1:0, captures the assigned port, and
// closes the listener — leaving the port free for ssh -L to bind
// almost immediately afterwards. There's a small race window;
// callers retry on failure.
func AllocPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
