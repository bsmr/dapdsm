// Package tunnel manages per-host SSH ControlMaster sessions and
// the local-port tunnel slots multiplexed over them.
package tunnel

import (
	"os"
	"path/filepath"
	"sync"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// RuntimeDir returns the dunemgr subdir under XDG_RUNTIME_DIR.
// Falls back to /tmp/dunemgr-<uid> when XDG_RUNTIME_DIR is unset.
func RuntimeDir() string {
	xdg := os.Getenv("XDG_RUNTIME_DIR")
	if xdg == "" {
		return filepath.Join(os.TempDir(), "dunemgr")
	}
	return filepath.Join(xdg, "dunemgr")
}

// SockPath returns the ControlMaster socket path for host.
func SockPath(host string) string {
	return filepath.Join(RuntimeDir(), "sock-"+host)
}

// active tracks one ControlMaster + its open slots.
type active struct {
	host  string
	slots map[string]int // target -> local port
}

// Manager owns ControlMaster lifecycle + slot catalog across all
// connected hosts. Methods are safe for concurrent use.
type Manager struct {
	SSH *ssh.Client

	mu     sync.Mutex
	byHost map[string]*active
}

// IsConnected returns true if the ControlMaster for host is up.
func (m *Manager) IsConnected(host string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.byHost[host]
	return ok
}
