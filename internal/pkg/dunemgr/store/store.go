// Package store wraps a bbolt database for dunemgr's persistent
// state. Buckets: hosts, audit, backups, schedules, statuscache, avatars.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

// allBuckets lists every top-level bucket. Order is stable so
// tests can iterate.
var allBuckets = []string{"hosts", "audit", "backups", "schedules", "statuscache", "avatars"}

// Store wraps a bbolt.DB.
type Store struct {
	db *bbolt.DB
}

// Open opens (or creates) the bbolt file at path with mode 0600
// and ensures all expected buckets exist. The parent directory is
// created if missing.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir parent: %w", err)
	}
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 3 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open state db %s: database locked (is dunemgr serve running?): %w", path, err)
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		for _, name := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close flushes and closes the underlying bbolt database.
func (s *Store) Close() error { return s.db.Close() }

// HasBucket returns true if a top-level bucket with the given name
// exists. Used by tests and bootstrap checks.
func (s *Store) HasBucket(name string) bool {
	found := false
	_ = s.db.View(func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte(name)) != nil {
			found = true
		}
		return nil
	})
	return found
}
