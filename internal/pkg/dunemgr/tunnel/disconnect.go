package tunnel

import (
	"context"
	"fmt"
)

// Disconnect closes the ControlMaster for host. All active slots
// on that master go down. No-op for hosts not currently connected.
func (m *Manager) Disconnect(ctx context.Context, host string) error {
	m.mu.Lock()
	_, exists := m.byHost[host]
	m.mu.Unlock()
	if !exists {
		return nil
	}
	if err := m.SSH.Disconnect(ctx, SockPath(host), host); err != nil {
		return fmt.Errorf("ssh disconnect: %w", err)
	}
	m.mu.Lock()
	delete(m.byHost, host)
	m.mu.Unlock()
	return nil
}
