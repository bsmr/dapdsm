// Package avatar implements per-character export / import / transfer on top of
// the Funcom-native dune.character_transfer_* functions, reached through the
// gamedb transport. It mirrors the backup package: managed records + files +
// audit, with --confirm gating destructive imports and a --check dry-run.
package avatar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

// DB is the subset of gamedb.Runner the avatar Runner needs. As an interface
// it lets tests inject a fake without re-faking the SSH wire. *gamedb.Runner
// satisfies it.
type DB interface {
	IsPlayerOffline(ctx context.Context, host, fls string) (bool, error)
	PatchesChecksum(ctx context.Context, host string) (string, error)
	CharacterName(ctx context.Context, host, fls string) (string, error)
	CharacterExport(ctx context.Context, host, fls string) (string, error)
	CharacterImport(ctx context.Context, host, dataJSON, fls, name string) (int64, error)
}

// Runner orchestrates avatar operations.
type Runner struct {
	DB      DB
	Store   *store.Store
	DataDir string // e.g. core.DataDir + "/avatars"
}

// PreflightReport records the outcome of each transfer gate.
type PreflightReport struct {
	SrcOffline    bool
	DstOffline    bool
	FLSExists     bool
	SrcChecksum   string
	DstChecksum   string
	ChecksumMatch bool
	CharacterName string
}

// OK reports whether all gates passed.
func (p PreflightReport) OK() bool {
	return p.SrcOffline && p.DstOffline && p.FLSExists && p.ChecksumMatch
}

// exportEnvelope is the subset of the Funcom export JSON we read.
type exportEnvelope struct {
	PatchesChecksum string `json:"_patches_checksum"`
	FuncomID        string `json:"funcom_id"`
}

func parseChecksum(dataJSON string) (string, error) {
	var env exportEnvelope
	if err := json.Unmarshal([]byte(dataJSON), &env); err != nil {
		return "", fmt.Errorf("parse export checksum: %w", err)
	}
	if env.PatchesChecksum == "" {
		return "", fmt.Errorf("parse export checksum: export is missing _patches_checksum")
	}
	return env.PatchesChecksum, nil
}

func (r *Runner) audit(operator, host, action, fls, result string) {
	_ = r.Store.AppendAudit(store.AuditEntry{
		Operator: operator,
		Host:     host,
		Action:   action,
		Subject:  "fls=" + fls,
		Result:   result,
	})
}

// Check runs all transfer pre-flight gates without mutating anything.
func (r *Runner) Check(ctx context.Context, src, dst, fls string) (*PreflightReport, error) {
	rep := &PreflightReport{}
	name, err := r.DB.CharacterName(ctx, src, fls)
	if err != nil {
		return nil, err
	}
	rep.CharacterName = name
	rep.FLSExists = name != ""

	if rep.SrcOffline, err = r.DB.IsPlayerOffline(ctx, src, fls); err != nil {
		return nil, err
	}
	if rep.DstOffline, err = r.DB.IsPlayerOffline(ctx, dst, fls); err != nil {
		return nil, err
	}
	if rep.SrcChecksum, err = r.DB.PatchesChecksum(ctx, src); err != nil {
		return nil, err
	}
	if rep.DstChecksum, err = r.DB.PatchesChecksum(ctx, dst); err != nil {
		return nil, err
	}
	rep.ChecksumMatch = rep.SrcChecksum != "" && rep.SrcChecksum == rep.DstChecksum
	return rep, nil
}

// Export dumps the avatar fls on host to a managed file + record. The player
// must be offline.
func (r *Runner) Export(ctx context.Context, operator, host, fls string) (store.ExportRecord, error) {
	offline, err := r.DB.IsPlayerOffline(ctx, host, fls)
	if err != nil {
		return store.ExportRecord{}, err
	}
	if !offline {
		return store.ExportRecord{}, fmt.Errorf("player %q is online; export requires the player to be offline", fls)
	}

	name, _ := r.DB.CharacterName(ctx, host, fls) // best-effort label

	data, err := r.DB.CharacterExport(ctx, host, fls)
	if err != nil {
		r.audit(operator, host, "avatar.export", fls, "error: "+err.Error())
		return store.ExportRecord{}, err
	}
	checksum, err := parseChecksum(data)
	if err != nil {
		r.audit(operator, host, "avatar.export", fls, "error: "+err.Error())
		return store.ExportRecord{}, err
	}

	now := time.Now().UTC()
	dir := filepath.Join(r.DataDir, host)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return store.ExportRecord{}, err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%d.json", fls, now.Unix()))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		return store.ExportRecord{}, err
	}

	rec := store.ExportRecord{
		Host:          host,
		FLSID:         fls,
		CharacterName: name,
		UnixTS:        now.Unix(),
		LocalPath:     path,
		Bytes:         int64(len(data)),
		Checksum:      checksum,
		Operator:      operator,
		CreatedAt:     now,
	}
	if err := r.Store.PutExport(rec); err != nil {
		return store.ExportRecord{}, err
	}
	r.audit(operator, host, "avatar.export", fls, "ok key="+rec.Key())
	return rec, nil
}
