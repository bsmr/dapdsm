package hostpool

import (
	"context"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
)

// Manager registers and tracks hosts. Wraps the store (persistent profiles).
type Manager struct {
	Store *store.Store
}

// Register validates the inputs and writes the HostProfile to the store.
// Replaces an existing entry with the same name. dunemgr reaches the host
// purely over SSH (kubectl runs on the node), so no cluster CA is fetched.
func (m *Manager) Register(_ context.Context, name, sshAlias string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := ValidateSSHAlias(sshAlias); err != nil {
		return err
	}
	return m.Store.PutHost(store.HostProfile{Name: name, SSHAlias: sshAlias})
}

// Get returns the host profile for name, or store.ErrNotFound.
func (m *Manager) Get(name string) (store.HostProfile, error) {
	return m.Store.GetHost(name)
}

// List returns all host profiles sorted by name.
func (m *Manager) List() ([]store.HostProfile, error) {
	return m.Store.ListHosts()
}

// Delete removes the profile by name.
func (m *Manager) Delete(name string) error {
	return m.Store.DeleteHost(name)
}
