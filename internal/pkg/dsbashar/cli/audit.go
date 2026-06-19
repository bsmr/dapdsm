package cli

import (
	"os"
	"path/filepath"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

// auditStorePath is the on-VM bbolt path for ds-bashar's audit log.
// Override with DS_BASHAR_STATE.
func auditStorePath() string {
	if p := os.Getenv("DS_BASHAR_STATE"); p != "" {
		return p
	}
	return "/home/dune/.dune/state/ds-bashar.bolt"
}

// openAuditStore opens the audit store best-effort. On failure it returns
// (nil, noop, err); callers may log and proceed with a nil Store (mq.Publisher
// tolerates a nil Store and simply skips auditing).
func openAuditStore() (*store.Store, func(), error) {
	path := auditStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, func() {}, err
	}
	st, err := store.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return st, func() { _ = st.Close() }, nil
}
