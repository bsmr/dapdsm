package tunnel

import (
	"context"
	"fmt"
	"os"
)

// Connect starts an SSH ControlMaster against host. Idempotent —
// calling Connect twice with the same host runs ssh only once.
// Creates the runtime dir if absent.
func (m *Manager) Connect(ctx context.Context, host string) error {
	m.mu.Lock()
	if _, exists := m.byHost[host]; exists {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	if err := os.MkdirAll(RuntimeDir(), 0o700); err != nil {
		return fmt.Errorf("mkdir runtime dir: %w", err)
	}
	sock := SockPath(host)
	if err := m.SSH.Connect(ctx, host, sock); err != nil {
		return fmt.Errorf("ssh ControlMaster: %w", err)
	}

	m.mu.Lock()
	if m.byHost == nil {
		m.byHost = map[string]*active{}
	}
	m.byHost[host] = &active{host: host, slots: map[string]int{}}
	m.mu.Unlock()
	return nil
}
