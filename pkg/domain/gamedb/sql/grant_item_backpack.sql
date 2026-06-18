WITH bp AS (
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
RETURNING id::bigint;