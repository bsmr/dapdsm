SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'dune' AND table_name = 'player_state'
ORDER BY ordinal_position;