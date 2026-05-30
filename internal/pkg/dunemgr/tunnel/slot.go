package tunnel

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// OpenSlot allocates a local 127.0.0.1 port and asks the ControlMaster
// to add an -L forward to targetHost:targetPort. If a slot for the
// same target is already open on this host, returns the existing
// local port (no new forward issued). Returns an error if host isn't
// connected.
func (m *Manager) OpenSlot(ctx context.Context, host, targetHost string, targetPort int) (int, error) {
	key := fmt.Sprintf("%s:%d", targetHost, targetPort)

	m.mu.Lock()
	act, ok := m.byHost[host]
	if !ok {
		m.mu.Unlock()
		return 0, fmt.Errorf("host %q: not connected", host)
	}
	if existing, found := act.slots[key]; found {
		m.mu.Unlock()
		return existing, nil
	}
	m.mu.Unlock()

	localPort, err := ssh.AllocPort()
	if err != nil {
		return 0, fmt.Errorf("alloc port: %w", err)
	}
	if err := m.SSH.OpenTunnel(ctx, host, SockPath(host), localPort, targetHost, targetPort); err != nil {
		return 0, fmt.Errorf("ssh forward: %w", err)
	}

	m.mu.Lock()
	act.slots[key] = localPort
	m.mu.Unlock()
	return localPort, nil
}

// SlotPort returns the cached local port for a previously opened
// slot, or 0 if none. Useful for callers that want to reuse without
// risking a re-Open round trip.
func (m *Manager) SlotPort(host, targetHost string, targetPort int) int {
	key := fmt.Sprintf("%s:%d", targetHost, targetPort)
	m.mu.Lock()
	defer m.mu.Unlock()
	act, ok := m.byHost[host]
	if !ok {
		return 0
	}
	return act.slots[key]
}
