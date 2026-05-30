package hostpool

import (
	"context"
	"fmt"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// Manager registers and tracks hosts. Combines the store
// (persistent profiles) with an SSH client (for K3s CA bootstrap).
type Manager struct {
	Store *store.Store
	SSH   *ssh.Client
}

// Register validates the inputs, fetches the K3s CA + FQDN from
// the target host via SSH, and writes the resulting HostProfile to
// the store. Replaces an existing entry with the same name.
func (m *Manager) Register(ctx context.Context, name, sshAlias string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := ValidateSSHAlias(sshAlias); err != nil {
		return err
	}
	caB64, fqdn, err := fetchK3sCA(ctx, m.SSH, sshAlias)
	if err != nil {
		return fmt.Errorf("fetch K3s CA: %w", err)
	}
	return m.Store.PutHost(store.HostProfile{
		Name:        name,
		SSHAlias:    sshAlias,
		FQDN:        fqdn,
		K3sCABase64: caB64,
	})
}

// Get returns the host profile for name, or store.ErrNotFound.
func (m *Manager) Get(name string) (store.HostProfile, error) {
	return m.Store.GetHost(name)
}

// List returns all host profiles sorted by name.
func (m *Manager) List() ([]store.HostProfile, error) {
	return m.Store.ListHosts()
}

// Delete removes the profile by name. Tunnels managed by the
// tunnel package must be closed separately by the caller.
func (m *Manager) Delete(name string) error {
	return m.Store.DeleteHost(name)
}
