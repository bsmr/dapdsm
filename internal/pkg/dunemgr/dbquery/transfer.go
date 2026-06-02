package dbquery

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// onErrorStop makes psql abort (non-zero exit) on a server-side RAISE from the
// Funcom transfer functions, instead of psql's default of swallowing the error
// (exit 0, empty stdout) — which would make IsPlayerOffline silently parse
// "offline=false". A backslash meta-command emits nothing to stdout.
const onErrorStop = "\\set ON_ERROR_STOP on\n"

// IsPlayerOffline reports whether fls is offline per dune.is_player_offline.
// The transfer functions require offline; this is a friendly pre-flight.
func (r *Runner) IsPlayerOffline(ctx context.Context, host, fls string) (bool, error) {
	res, err := r.execWithVars(ctx, host,
		onErrorStop+`SELECT dune.is_player_offline(:'fls');`,
		map[string]string{"fls": fls})
	if err != nil {
		return false, fmt.Errorf("is_player_offline: %w", err)
	}
	return strings.TrimSpace(res.Stdout) == "t", nil
}

// PatchesChecksum returns the target DB's character-transfer patches checksum.
// Source and destination must match for an import to succeed.
func (r *Runner) PatchesChecksum(ctx context.Context, host string) (string, error) {
	res, err := r.execNoAudit(ctx, host,
		onErrorStop+`SELECT dune._character_transfer_get_patches_checksum();`)
	if err != nil {
		return "", fmt.Errorf("patches checksum: %w", err)
	}
	return strings.TrimSpace(res.Stdout), nil
}

// CharacterName returns the plaintext player_state.character_name for fls, or
// "" if no such player exists. Used as the default import name (the in-dump
// character name is encrypted and not recoverable).
func (r *Runner) CharacterName(ctx context.Context, host, fls string) (string, error) {
	const sql = onErrorStop + `SELECT COALESCE(ps.character_name,'')
FROM dune.player_state ps
JOIN dune.accounts a ON a.id = ps.account_id
WHERE a."user"::text = :'fls' LIMIT 1;`
	res, err := r.execWithVars(ctx, host, sql, map[string]string{"fls": fls})
	if err != nil {
		return "", fmt.Errorf("character name: %w", err)
	}
	return strings.TrimSpace(res.Stdout), nil
}

// CharacterExport dumps the avatar fls to its Funcom transfer JSON
// (dune.character_transfer_export(text)::text). The player must be offline
// (the DB raises otherwise).
func (r *Runner) CharacterExport(ctx context.Context, host, fls string) (string, error) {
	res, err := r.execWithVars(ctx, host,
		onErrorStop+`SELECT dune.character_transfer_export(:'fls')::text;`,
		map[string]string{"fls": fls})
	if err != nil {
		return "", fmt.Errorf("character export: %w", err)
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return "", fmt.Errorf("character export: empty result for fls %q", fls)
	}
	return out, nil
}

// CharacterImport restores dataJSON into the account fls under the given
// character name (dune.character_transfer_import). DESTRUCTIVE: the DB function
// deletes the current account first. Returns the new player_controller_id.
//
// dataJSON (potentially hundreds of KB, machine-generated, trusted) is base64-
// encoded and embedded in a $b64$ dollar-quoted literal carried over stdin, so
// it is never an argv element (avoids ARG_MAX) and can never break out of its
// quote (the base64 alphabet contains no '$'). fls and name — the only
// operator-controlled values — are bound as psql variables.
func (r *Runner) CharacterImport(ctx context.Context, host, dataJSON, fls, name string) (int64, error) {
	b64 := base64.StdEncoding.EncodeToString([]byte(dataJSON))
	sql := onErrorStop + `SELECT dune.character_transfer_import(
  convert_from(decode($b64$` + b64 + `$b64$, 'base64'), 'UTF8')::jsonb,
  :'fls', :'name');`
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"fls":  fls,
		"name": name,
	})
	if err != nil {
		return 0, fmt.Errorf("character import: %w", err)
	}
	out := strings.TrimSpace(res.Stdout)
	id, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("character import: unexpected result %q: %w", out, err)
	}
	return id, nil
}
