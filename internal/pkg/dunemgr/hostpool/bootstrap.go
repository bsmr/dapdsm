package hostpool

import (
	"context"
	"fmt"
	"regexp"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

var (
	reCAData = regexp.MustCompile(`certificate-authority-data:\s*([A-Za-z0-9+/=_-]+)`)
	reServer = regexp.MustCompile(`server:\s*https?://([^:/\s]+)`)
)

// fetchK3sCA reads /etc/rancher/k3s/k3s.yaml from host (over SSH,
// no sudo — installer mode 0644 makes it world-readable) and
// extracts the base64-encoded CA plus the FQDN from the server
// URL.
func fetchK3sCA(ctx context.Context, client *ssh.Client, host string) (caB64, fqdn string, err error) {
	res, err := client.Run(ctx, host, "cat", "/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return "", "", fmt.Errorf("ssh cat k3s.yaml: %w", err)
	}
	caMatch := reCAData.FindStringSubmatch(res.Stdout)
	if caMatch == nil {
		return "", "", fmt.Errorf("certificate-authority-data not found in k3s.yaml")
	}
	srvMatch := reServer.FindStringSubmatch(res.Stdout)
	if srvMatch == nil {
		return "", "", fmt.Errorf("server URL not found in k3s.yaml")
	}
	return caMatch[1], srvMatch[1], nil
}
