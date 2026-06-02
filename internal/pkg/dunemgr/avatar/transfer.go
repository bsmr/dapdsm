package avatar

import (
	"context"
	"fmt"
	"os"
)

// TransferResult summarises a completed transfer.
type TransferResult struct {
	ExportKey    string
	ControllerID int64
	Report       *PreflightReport
}

// Import restores the export identified by key into the account fls on host.
// DESTRUCTIVE: overwrites the current avatar (the DB deletes it first), so
// confirm must be true. If name is empty, the record's stored CharacterName is
// used. Returns the new player_controller_id.
func (r *Runner) Import(ctx context.Context, operator, host, fls, key, name string, confirm bool) (int64, error) {
	if !confirm {
		return 0, fmt.Errorf("import is destructive (overwrites the avatar on %s); pass --confirm", host)
	}
	rec, err := r.Store.GetExport(key)
	if err != nil {
		return 0, fmt.Errorf("export %q: %w", key, err)
	}
	if name == "" {
		name = rec.CharacterName
	}
	if name == "" {
		return 0, fmt.Errorf("no character name: the export record has none — pass --name")
	}

	data, err := os.ReadFile(rec.LocalPath)
	if err != nil {
		return 0, fmt.Errorf("read export %q: %w", rec.LocalPath, err)
	}

	offline, err := r.DB.IsPlayerOffline(ctx, host, fls)
	if err != nil {
		return 0, err
	}
	if !offline {
		return 0, fmt.Errorf("target player %q is online; import requires the player to be offline", fls)
	}

	dstChecksum, err := r.DB.PatchesChecksum(ctx, host)
	if err != nil {
		return 0, err
	}
	// rec.Checksum is the _patches_checksum embedded in the export blob — the
	// authoritative source-side value; Transfer's Check compares live DB
	// checksums, this is the real backstop at import time.
	if rec.Checksum != "" && rec.Checksum != dstChecksum {
		return 0, fmt.Errorf("patches-checksum mismatch: export %s != target %s; builds must match", rec.Checksum, dstChecksum)
	}

	id, err := r.DB.CharacterImport(ctx, host, string(data), fls, name)
	if err != nil {
		r.audit(operator, host, "avatar.import", fls, "error: "+err.Error())
		return 0, err
	}
	r.audit(operator, host, "avatar.import", fls, fmt.Sprintf("ok controller=%d key=%s", id, key))
	return id, nil
}

// Transfer moves the avatar fls from src to dst: it exports from src to a
// managed file, then imports that file into dst. check==true runs the
// pre-flight gates only (no export, no mutation). The real run is destructive
// on dst, so confirm must be true. name defaults to the source player's name.
func (r *Runner) Transfer(ctx context.Context, operator, src, dst, fls, name string, check, confirm bool) (*TransferResult, error) {
	rep, err := r.Check(ctx, src, dst, fls)
	if err != nil {
		return nil, err
	}
	if check {
		return &TransferResult{Report: rep}, nil
	}
	if !rep.SrcOffline {
		return nil, fmt.Errorf("source player %q is online; transfer requires offline on both ends", fls)
	}
	if !rep.DstOffline {
		return nil, fmt.Errorf("destination player %q is online; transfer requires offline on both ends", fls)
	}
	if !rep.FLSExists {
		return nil, fmt.Errorf("no player with fls %q on source %s", fls, src)
	}
	if !rep.ChecksumMatch {
		return nil, fmt.Errorf("patches-checksum mismatch: src %s != dst %s; builds must match", rep.SrcChecksum, rep.DstChecksum)
	}
	if !confirm {
		return nil, fmt.Errorf("transfer is destructive on %s; pass --confirm (or --check to dry-run)", dst)
	}
	if name == "" {
		name = rep.CharacterName
	}

	rec, err := r.Export(ctx, operator, src, fls)
	if err != nil {
		return nil, fmt.Errorf("transfer export: %w", err)
	}
	id, err := r.Import(ctx, operator, dst, fls, rec.Key(), name, true)
	if err != nil {
		return nil, fmt.Errorf("transfer import (export saved at %s): %w", rec.LocalPath, err)
	}
	r.audit(operator, dst, "avatar.transfer", fls, fmt.Sprintf("ok from=%s controller=%d key=%s", src, id, rec.Key()))
	return &TransferResult{ExportKey: rec.Key(), ControllerID: id, Report: rep}, nil
}
