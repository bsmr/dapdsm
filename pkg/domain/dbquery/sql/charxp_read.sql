SELECT
  (fe.components->'FLevelComponent'->1->>'TotalXPEarned')::bigint,
  COALESCE((SELECT SUM((v->>'SkillPointsSpent')::int)
     FROM jsonb_each(fe.components->'FLevelComponent'->1->'ModuleData') AS kv(k, v)
     WHERE k != format('(TagName="%s")', fe.components->'FLevelComponent'->1->'StarterSkillTreeTag'->>'TagName')), 0),
  ps.player_pawn_id, ps.player_controller_id
FROM dune.fgl_entities fe
JOIN dune.actor_fgl_entities afe ON afe.entity_id = fe.entity_id
JOIN dune.player_state ps ON ps.player_pawn_id = afe.actor_id
JOIN dune.accounts a ON a.id = ps.account_id
WHERE afe.slot_name = 'DuneCharacter' AND a."user"::text = :'fls' ORDER BY afe.entity_id DESC LIMIT 1;