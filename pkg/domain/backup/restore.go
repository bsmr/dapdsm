package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

// Restore pushes the local .backup pair back to the host, then runs
// `<bin> import <name>` over SSH. confirm must be true; otherwise
// the call returns an error and no state changes.
func (r *Runner) Restore(ctx context.Context, operator, recordKey string, confirm bool) error {
	if !confirm {
		return errors.New("backup.Restore: confirm=false; restore is destructive — require explicit confirmation")
	}
	rec, err := r.Store.GetBackup(recordKey)
	if err != nil {
		return fmt.Errorf("backup.Restore: %w", err)
	}

	audit := store.AuditEntry{
		Operator: operator,
		Host:     rec.Host,
		Action:   "backup.restore",
		Subject:  fmt.Sprintf("host=%s bg=%s name=%s key=%s", rec.Host, rec.BG, rec.Name, recordKey),
	}

	bin := r.Bin
	if bin == "" {
		bin = DefaultBattlegroupBin
	}
	hostBackupDir := r.HostBackupDir
	if hostBackupDir == "" {
		hostBackupDir = DefaultHostBackupDir
	}

	// 0. Sanity check both local files exist
	if _, err := os.Stat(rec.LocalPath); err != nil {
		audit.Result = "error: local .backup missing: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return fmt.Errorf("local .backup missing: %w", err)
	}
	if _, err := os.Stat(rec.LocalPath + ".yaml"); err != nil {
		audit.Result = "error: local .backup.yaml missing: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return fmt.Errorf("local .backup.yaml missing: %w", err)
	}

	// 1. SCP push pair
	remoteBackup, remoteYAML := RemotePair(hostBackupDir, rec.BG, rec.Name)
	if err := r.SSH.SendFile(ctx, rec.Host, rec.LocalPath, remoteBackup); err != nil {
		audit.Result = "error: scp .backup up: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return fmt.Errorf("scp .backup: %w", err)
	}
	if err := r.SSH.SendFile(ctx, rec.Host, rec.LocalPath+".yaml", remoteYAML); err != nil {
		audit.Result = "error: scp .backup.yaml up: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return fmt.Errorf("scp .backup.yaml: %w", err)
	}

	// 2. Run wrapper import
	res, err := r.SSH.Run(ctx, rec.Host, bin, "import", rec.Name)
	if err != nil || res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		audit.Result = fmt.Sprintf("error: import: exit=%d %s", res.ExitCode, msg)
		_ = r.Store.AppendAudit(audit)
		return fmt.Errorf("battlegroup import %s on %s: exit %d: %s", rec.Name, rec.Host, res.ExitCode, msg)
	}
	audit.Result = "ok"
	_ = r.Store.AppendAudit(audit)
	return nil
}
