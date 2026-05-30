package ssh

import "context"

// RecvFile scp's host:remotePath down to localPath on the
// workstation. Auth/jump handling is the system scp binary's job.
func (c *Client) RecvFile(ctx context.Context, host, remotePath, localPath string) error {
	if err := mustNotBeFlag("host", host); err != nil {
		return err
	}
	if err := mustNotBeFlag("remotePath", remotePath); err != nil {
		return err
	}
	if err := mustNotBeFlag("localPath", localPath); err != nil {
		return err
	}
	src := host + ":" + remotePath
	_, err := c.runner().Run(ctx, "scp", "-o", "BatchMode=yes", "--", src, localPath)
	return err
}
