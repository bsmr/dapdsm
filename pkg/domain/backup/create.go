package backup

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

// Create runs `<bin> backup <name>` on the host and SCP-pulls both
// the .backup and .backup.yaml files into the local data dir. A
// store.BackupRecord is appended on success only.
func (r *Runner) Create(ctx context.Context, operator, host, bg, name string) (*store.BackupRecord, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	if err := ValidateName(host); err != nil {
		return nil, err
	}
	if err := ValidateName(bg); err != nil {
		return nil, err
	}

	audit := store.AuditEntry{
		Operator: operator,
		Host:     host,
		Action:   "backup.create",
		Subject:  fmt.Sprintf("host=%s bg=%s name=%s", host, bg, name),
	}

	bin := r.Bin
	if bin == "" {
		bin = DefaultBattlegroupBin
	}
	hostBackupDir := r.HostBackupDir
	if hostBackupDir == "" {
		hostBackupDir = DefaultHostBackupDir
	}

	// 1. Run wrapper backup.
	res, err := r.SSH.Run(ctx, host, bin, "backup", name)
	if err != nil || res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		audit.Result = fmt.Sprintf("error: wrapper: exit=%d %s err=%v", res.ExitCode, msg, err)
		_ = r.Store.AppendAudit(audit)
		return nil, fmt.Errorf("battlegroup backup %s on %s: exit %d: %s", name, host, res.ExitCode, msg)
	}

	// 2. SCP pull pair.
	unixTS := time.Now().Unix()
	localBackupPath, err := LocalPair(r.DataDir, host, bg, unixTS, name)
	if err != nil {
		audit.Result = "error: local pair: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return nil, err
	}
	localYAMLPath := localBackupPath + ".yaml"
	remoteBackup, remoteYAML := RemotePair(hostBackupDir, bg, name)

	if err := r.SSH.RecvFile(ctx, host, remoteBackup, localBackupPath); err != nil {
		audit.Result = "error: scp .backup: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return nil, fmt.Errorf("scp .backup: %w", err)
	}
	if err := r.SSH.RecvFile(ctx, host, remoteYAML, localYAMLPath); err != nil {
		// Roll back the .backup we just pulled to avoid orphaned files.
		_ = os.Remove(localBackupPath)
		audit.Result = "error: scp .backup.yaml: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return nil, fmt.Errorf("scp .backup.yaml: %w", err)
	}

	// 3. Stat + store record.
	bSt, _ := os.Stat(localBackupPath)
	ySt, _ := os.Stat(localYAMLPath)
	rec := store.BackupRecord{
		Host:      host,
		BG:        bg,
		Name:      name,
		UnixTS:    unixTS,
		LocalPath: localBackupPath,
		Bytes:     statSize(bSt),
		YAMLBytes: statSize(ySt),
		Operator:  operator,
		CreatedAt: time.Unix(unixTS, 0).UTC(),
	}
	if err := r.Store.PutBackup(rec); err != nil {
		audit.Result = "error: store: " + err.Error()
		_ = r.Store.AppendAudit(audit)
		return nil, err
	}
	audit.Result = "ok"
	_ = r.Store.AppendAudit(audit)
	return &rec, nil
}

func statSize(st os.FileInfo) int64 {
	if st == nil {
		return 0
	}
	return st.Size()
}
