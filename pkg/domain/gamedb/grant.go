package gamedb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// GrantCurrency calls the Funcom virtual-currency primitive for the player's
// controller, returning the new balance. The function is DB-authoritative only
// when the player is offline; gating is the caller's responsibility.
func (r *Runner) GrantCurrency(ctx context.Context, host, fls string, currencyID int, delta int64) (int64, error) {
	sql := q("grant_currency")
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"fls": fls, "currency": strconv.Itoa(currencyID), "delta": strconv.FormatInt(delta, 10),
	})
	if err != nil {
		return 0, fmt.Errorf("grant currency: %w", err)
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return 0, fmt.Errorf("grant currency: no balance returned (unknown fls or controller)")
	}
	bal, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("grant currency: parse balance %q: %w", out, err)
	}
	return bal, nil
}

// GrantItemDB inserts one stack of template (size count, quality) into the
// player's backpack (inventory_type 0) at the first free slot, mirroring the
// ddsm welcome-package insert. Returns the new item id. An empty RETURNING
// (no backpack or no free slot) is an error.
func (r *Runner) GrantItemDB(ctx context.Context, host, fls, template string, count, quality int64) (int64, error) {
	sql := q("grant_item_backpack")
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"fls": fls, "template": template,
		"count": strconv.FormatInt(count, 10), "quality": strconv.FormatInt(quality, 10),
	})
	if err != nil {
		return 0, fmt.Errorf("grant item (db): %w", err)
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return 0, fmt.Errorf("grant item (db): nothing inserted (no backpack or no free slot for fls)")
	}
	// A psql INSERT … RETURNING prints the id on the first line, then the
	// command tag (e.g. "INSERT 0 1") on the next — take only the id line.
	if i := strings.IndexByte(out, '\n'); i >= 0 {
		out = strings.TrimSpace(out[:i])
	}
	id, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("grant item (db): parse id %q: %w", out, err)
	}
	return id, nil
}

// PersonaSeed is the fixed identity written for a reserved GM/Server chat persona.
// Mirrors dune-admin's verified gmSeed: BASE-table rows (dune.accounts/player_state
// are decrypting VIEWS) plus three linked actor rows the game's player-info lookup
// needs. Names go through dune.encrypt_user_data so it stays correct if user-data
// encryption is enabled. actors.transform is left NULL so the persona never plots on
// the map. online_status defaults Offline (blast-radius safe).
type PersonaSeed struct {
	AccountID                              int64
	ControllerID, StateID, PawnID          int64
	HexID, FuncomID, CharacterName         string
	ControllerClass, StateClass, PawnClass string
	Map                                    string
	PartitionID                            int64
	DimensionIndex                         int
	LifeState, OnlineStatus                string
}

// SeedPersona idempotently writes the persona's base-table identity in one
// transaction (ON CONFLICT DO NOTHING). Safe to call repeatedly.
func (r *Runner) SeedPersona(ctx context.Context, host string, s PersonaSeed) error {
	sql := onErrorStop + q("grant_persona_seed")
	vars := map[string]string{
		"acct": strconv.FormatInt(s.AccountID, 10), "ctrl": strconv.FormatInt(s.ControllerID, 10),
		"state": strconv.FormatInt(s.StateID, 10), "pawn": strconv.FormatInt(s.PawnID, 10),
		"hex": s.HexID, "funcom": s.FuncomID, "name": s.CharacterName,
		"cclass": s.ControllerClass, "sclass": s.StateClass, "pclass": s.PawnClass,
		"map": s.Map, "part": strconv.FormatInt(s.PartitionID, 10),
		"dim": strconv.Itoa(s.DimensionIndex), "life": s.LifeState, "online": s.OnlineStatus,
	}
	if _, err := r.execWithVars(ctx, host, sql, vars); err != nil {
		return fmt.Errorf("seed persona %s: %w", s.HexID, err)
	}
	return nil
}

// GrantSkillpoints adds amount to UnspentSkillPoints only (additive) in the
// character's FLevelComponent, returning the new unspent total. The 0.1.8 Total
// bump is intentionally dropped so the offline (DB) and online (MQ
// SkillsSetUnspentSkillPoints) paths touch the same field. DB-authoritative only
// when the player is offline; gating is the caller's responsibility.
func (r *Runner) GrantSkillpoints(ctx context.Context, host, fls string, amount int64) (int64, error) {
	sql := q("grant_skillpoints_update")
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"fls": fls, "amount": strconv.FormatInt(amount, 10),
	})
	if err != nil {
		return 0, fmt.Errorf("grant skillpoints: %w", err)
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return 0, fmt.Errorf("grant skillpoints: no character updated (unknown fls)")
	}
	// A psql UPDATE … RETURNING prints the value on the first line, then the
	// command tag (e.g. "UPDATE 1") on the next — take only the value line.
	if i := strings.IndexByte(out, '\n'); i >= 0 {
		out = strings.TrimSpace(out[:i])
	}
	n, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("grant skillpoints: parse unspent %q: %w", out, err)
	}
	return n, nil
}

// GrantTrackXP adds amount to a specialization track's xp_amount (clamped to
// [0, 44182]), inserting the track row if absent — mirroring dune-admin's
// cmdAwardXP. Returns the new xp_amount. DB-authoritative only when offline.
func (r *Runner) GrantTrackXP(ctx context.Context, host, fls, track string, amount int64) (int64, error) {
	sql := onErrorStop + q("grant_skillpoints")
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"fls": fls, "track": track, "amount": strconv.FormatInt(amount, 10),
	})
	if err != nil {
		return 0, fmt.Errorf("grant track xp: %w", err)
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return 0, fmt.Errorf("grant track xp: nothing updated (unknown fls or controller)")
	}
	n, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("grant track xp: parse %q: %w", out, err)
	}
	return n, nil
}

// UnspentSkillpoints reads the character's current UnspentSkillPoints (0 if the
// field is absent). Used by the online give-skillpoints path to compute the
// absolute MQ target (base + delta). The base may lag the live game (the engine
// writes FLevelComponent back on logout/sync).
func (r *Runner) UnspentSkillpoints(ctx context.Context, host, fls string) (int64, error) {
	sql := q("grant_skillpoints_balance")
	res, err := r.execWithVars(ctx, host, sql, map[string]string{"fls": fls})
	if err != nil {
		return 0, fmt.Errorf("read unspent skillpoints: %w", err)
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return 0, fmt.Errorf("read unspent skillpoints: unknown fls")
	}
	n, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("read unspent skillpoints: parse %q: %w", out, err)
	}
	return n, nil
}
