SELECT COALESCE(ps.character_name,'')
FROM dune.player_state ps
JOIN dune.accounts a ON a.id = ps.account_id
WHERE a."user"::text = :'fls' LIMIT 1;