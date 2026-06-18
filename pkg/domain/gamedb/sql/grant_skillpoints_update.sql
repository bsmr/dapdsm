UPDATE dune.fgl_entities fe
SET components = jsonb_set(fe.components, '{FLevelComponent,1,UnspentSkillPoints}',
      to_jsonb(COALESCE((fe.components #>> '{FLevelComponent,1,UnspentSkillPoints}')::bigint,0) + :amount::bigint))
FROM dune.actor_fgl_entities afe
JOIN dune.player_state ps ON ps.player_pawn_id = afe.actor_id
JOIN dune.accounts a ON a.id = ps.account_id
WHERE fe.entity_id = afe.entity_id AND afe.slot_name = 'DuneCharacter' AND a."user"::text = :'fls'
RETURNING (fe.components #>> '{FLevelComponent,1,UnspentSkillPoints}')::bigint;