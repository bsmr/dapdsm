package store

import (
	"encoding/json"
	"errors"
	"fmt"

	"go.etcd.io/bbolt"
)

// HostProfile holds everything dunemgr needs to talk to one host.
// Stored as JSON in the "hosts" bucket, keyed by Name. Extra keys on
// older records (e.g. fqdn, k3s_ca_b64) are ignored on load.
type HostProfile struct {
	Name     string `json:"name"`      // operator-chosen label
	SSHAlias string `json:"ssh_alias"` // resolved by ~/.ssh/config
}

// ErrNotFound is returned by Get* when the key is absent.
var ErrNotFound = errors.New("not found")

// PutHost stores or replaces a host profile.
func (s *Store) PutHost(h HostProfile) error {
	data, err := json.Marshal(h)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("hosts")).Put([]byte(h.Name), data)
	})
}

// GetHost returns the named host profile, or ErrNotFound.
func (s *Store) GetHost(name string) (HostProfile, error) {
	var h HostProfile
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte("hosts")).Get([]byte(name))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &h)
	})
	return h, err
}

// ListHosts returns all host profiles sorted by Name.
// (bbolt cursors iterate keys in byte order; profile Name is the
// key, so sort-by-name is free.)
func (s *Store) ListHosts() ([]HostProfile, error) {
	var out []HostProfile
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte("hosts")).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var h HostProfile
			if err := json.Unmarshal(v, &h); err != nil {
				return fmt.Errorf("unmarshal %q: %w", k, err)
			}
			out = append(out, h)
		}
		return nil
	})
	return out, err
}

// DeleteHost removes a host profile by name. No error if absent.
func (s *Store) DeleteHost(name string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("hosts")).Delete([]byte(name))
	})
}
