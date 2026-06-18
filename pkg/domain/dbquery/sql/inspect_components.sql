SELECT fe.components
FROM dune.fgl_entities fe
JOIN dune.actor_fgl_entities afe ON afe.entity_id = fe.entity_id
JOIN dune.player_state ps ON ps.player_pawn_id = afe.actor_id
JOIN dune.accounts a ON a.id = ps.account_id
WHERE a."user"::text = :'fls' AND afe.slot_name = 'DuneCharacter'
LIMIT 1;