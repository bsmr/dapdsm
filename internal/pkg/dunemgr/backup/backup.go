// Package backup orchestrates Funcom's `battlegroup backup` / `import`
// flow: the host runs the wrapper, the operator workstation SCP-pulls
// the resulting pair, store records the local path.
package backup

import (
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// DefaultBattlegroupBin matches lifecycle.DefaultBattlegroupBin.
const DefaultBattlegroupBin = "/home/dune/.dune/bin/battlegroup"

// DefaultHostBackupDir is where Funcom's database-operator writes
// backup pairs: /funcom/artifacts/database-dumps/<bg>/<name>.{backup,backup.yaml}.
const DefaultHostBackupDir = "/funcom/artifacts/database-dumps"

// Runner orchestrates create / list / restore against a single
// dunemgr store + ssh client. DataDir is the workstation-side
// root under which `<host>/<bg>/` directories appear.
type Runner struct {
	SSH     *ssh.Client
	Store   *store.Store
	DataDir string
	// Bin overrides the on-host Funcom wrapper path. Empty = default.
	Bin string
	// HostBackupDir overrides the on-host backup directory. Empty = default.
	HostBackupDir string
}
