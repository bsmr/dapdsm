SELECT COALESCE((fe.components #>> '{FLevelComponent,1,UnspentSkillPoints}')::bigint, 0)
FROM dune.fgl_entities fe
JOIN dune.actor_fgl_entities afe ON afe.entity_id = fe.entity_id
JOIN dune.player_state ps ON ps.player_pawn_id = afe.actor_id
JOIN dune.accounts a ON a.id = ps.account_id
WHERE afe.slot_name = 'DuneCharacter' AND a."user"::text = :'fls'
LIMIT 1;