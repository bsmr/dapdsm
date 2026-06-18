package ssh

import "context"

// SendFile scp's localPath to host:remotePath. Auth/jump handling
// is the system scp binary's job.
func (c *Client) SendFile(ctx context.Context, host, localPath, remotePath string) error {
	if err := mustNotBeFlag("host", host); err != nil {
		return err
	}
	if err := mustNotBeFlag("localPath", localPath); err != nil {
		return err
	}
	if err := mustNotBeFlag("remotePath", remotePath); err != nil {
		return err
	}
	dst := host + ":" + remotePath
	_, err := c.runner().Run(ctx, "scp", "-o", "BatchMode=yes", "--", localPath, dst)
	return err
}
