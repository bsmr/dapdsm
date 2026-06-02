package dbquery

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
	const sql = `SELECT dune.adjust_player_virtual_currency_balance(
  (SELECT ps.player_controller_id FROM dune.player_state ps
     JOIN dune.accounts a ON a.id = ps.account_id
     WHERE a."user"::text = :'fls' LIMIT 1),
  :currency::smallint, :delta::bigint);`
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
	const sql = `WITH bp AS (
  SELECT inv.id AS inventory_id
  FROM dune.player_state ps
  JOIN dune.accounts a ON a.id = ps.account_id
  JOIN dune.actors pawn ON pawn.id = ps.player_pawn_id
  JOIN dune.inventories inv ON inv.actor_id = ps.player_pawn_id AND inv.inventory_type = 0
  WHERE a."user"::text = :'fls'
    AND pawn.class = '/Game/Dune/Characters/Player/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C'
  ORDER BY ps.last_login_time DESC NULLS LAST, inv.id DESC
  LIMIT 1
),
// slot: first free position in the backpack. generate_series caps the scan at
// 10000 (far above any real backpack); a full backpack yields no row → error.
slot AS (
  SELECT gs::bigint AS position_index
  FROM generate_series(0, 10000) gs, bp
  WHERE NOT EXISTS (
    SELECT 1 FROM dune.items i WHERE i.inventory_id = bp.inventory_id AND i.position_index = gs
  )
  ORDER BY gs LIMIT 1
)
INSERT INTO dune.items (inventory_id, stack_size, position_index, template_id, is_new, acquisition_time, stats, quality_level)
SELECT bp.inventory_id, :count::bigint, slot.position_index, :'template', TRUE, EXTRACT(EPOCH FROM now())::bigint, '{}'::jsonb, :quality::bigint
FROM bp, slot
RETURNING id::bigint;`
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
	const sql = onErrorStop + `BEGIN;
INSERT INTO dune.encrypted_accounts (id, "user", encrypted_funcom_id, takeoverable, platform_id, platform_name)
VALUES (:acct, :'hex', dune.encrypt_user_data(:'funcom'), false, 'dunemgr', 'Dunemgr')
ON CONFLICT DO NOTHING;
INSERT INTO dune.actors (id, class, map, partition_id, dimension_index, gas_attributes, properties, owner_account_id, serial)
VALUES (:ctrl, :'cclass', :'map', :part, :dim, '{}'::jsonb, '{}'::jsonb, :acct, 1) ON CONFLICT DO NOTHING;
INSERT INTO dune.actors (id, class, map, partition_id, dimension_index, gas_attributes, properties, owner_account_id, serial)
VALUES (:state, :'sclass', :'map', :part, :dim, '{}'::jsonb, '{}'::jsonb, :acct, 1) ON CONFLICT DO NOTHING;
INSERT INTO dune.actors (id, class, map, partition_id, dimension_index, gas_attributes, properties, owner_account_id, serial)
VALUES (:pawn, :'pclass', :'map', :part, :dim, '{}'::jsonb, '{}'::jsonb, :acct, 1) ON CONFLICT DO NOTHING;
INSERT INTO dune.encrypted_player_state
  (account_id, encrypted_character_name, life_state, online_status, is_coriolis_processed,
   server_id, player_controller_id, player_pawn_id, player_state_id, last_login_time)
VALUES (:acct, dune.encrypt_user_data(:'name'), :'life'::dune.playerlifestate, :'online'::dune.playerconnectionstatus, false,
   (SELECT server_id FROM dune.encrypted_player_state WHERE server_id IS NOT NULL LIMIT 1),
   :ctrl, :pawn, :state, now())
ON CONFLICT DO NOTHING;
COMMIT;`
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

// GrantSkillpoints increments TotalSkillPoints and UnspentSkillPoints in the
// character's FLevelComponent by amount (the ng_da_webadmin recipe). Returns the
// new unspent total. DB-authoritative only when the player is offline.
func (r *Runner) GrantSkillpoints(ctx context.Context, host, fls string, amount int64) (int64, error) {
	const sql = `UPDATE dune.fgl_entities fe
SET components = jsonb_set(
      jsonb_set(fe.components, '{FLevelComponent,1,TotalSkillPoints}',
        to_jsonb(COALESCE((fe.components #>> '{FLevelComponent,1,TotalSkillPoints}')::bigint,0) + :amount::bigint)),
      '{FLevelComponent,1,UnspentSkillPoints}',
      to_jsonb(COALESCE((fe.components #>> '{FLevelComponent,1,UnspentSkillPoints}')::bigint,0) + :amount::bigint))
FROM dune.actor_fgl_entities afe
JOIN dune.player_state ps ON ps.player_pawn_id = afe.actor_id
JOIN dune.accounts a ON a.id = ps.account_id
WHERE fe.entity_id = afe.entity_id AND afe.slot_name = 'DuneCharacter' AND a."user"::text = :'fls'
RETURNING (fe.components #>> '{FLevelComponent,1,UnspentSkillPoints}')::bigint;`
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
	n, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("grant skillpoints: parse unspent %q: %w", out, err)
	}
	return n, nil
}
