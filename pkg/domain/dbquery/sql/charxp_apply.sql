BEGIN;
UPDATE dune.fgl_entities
SET components = jsonb_set(jsonb_set(jsonb_set(components,
    '{FLevelComponent,1,TotalXPEarned}',     to_jsonb(:newxp::bigint)),
    '{FLevelComponent,1,TotalSkillPoints}',  to_jsonb(:newtotal::bigint)),
    '{FLevelComponent,1,UnspentSkillPoints}', to_jsonb(:newunspent::bigint))
WHERE entity_id = (SELECT entity_id FROM dune.actor_fgl_entities
                   WHERE actor_id = :pawn::bigint AND slot_name = 'DuneCharacter');
UPDATE dune.actors
SET properties = jsonb_set(properties,
    '{TechKnowledgePlayerComponent,m_TechKnowledgePoints}', to_jsonb(:newintel::bigint))
WHERE id = :pawn::bigint AND properties ? 'TechKnowledgePlayerComponent';
COMMIT;