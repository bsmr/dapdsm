package backup

import (
	"context"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

// List returns all known backups for a host+bg pair (newest first).
// No SSH happens — this reads bbolt only.
func (r *Runner) List(ctx context.Context, host, bg string) ([]store.BackupRecord, error) {
	if err := ValidateName(host); err != nil {
		return nil, err
	}
	if err := ValidateName(bg); err != nil {
		return nil, err
	}
	return r.Store.ListBackups(host, bg)
}
