// Package publicip resolves the IPv4 address external clients use to
// reach this host. Used by dunectl to populate settings.conf and the
// HOST_DATACENTER_IP_ADDRESS env-var without operator input.
package publicip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// Resolver returns the host's public IPv4.
type Resolver interface {
	Resolve(ctx context.Context) (string, error)
}

// DefaultURL is the endpoint HTTPResolver hits when URL is empty.
const DefaultURL = "https://api.ipify.org"

// HTTPResolver fetches the IP from a plain-text HTTP endpoint and
// validates that the response is a single IPv4 dotted-quad.
type HTTPResolver struct {
	// URL is the endpoint to GET. Defaults to DefaultURL.
	URL string
	// Client overrides the HTTP client. Defaults to a 10s-timeout client.
	Client *http.Client
}

func (r *HTTPResolver) Resolve(ctx context.Context) (string, error) {
	url := r.URL
	if url == "" {
		url = DefaultURL
	}
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	return validateIPv4(strings.TrimSpace(string(body)))
}

func validateIPv4(s string) (string, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return "", fmt.Errorf("response %q is not a valid IP address", s)
	}
	if !addr.Is4() {
		return "", fmt.Errorf("response %q is not IPv4", s)
	}
	return s, nil
}
