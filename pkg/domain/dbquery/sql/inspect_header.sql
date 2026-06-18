SELECT COALESCE(ps.character_name,''), COALESCE(ps.online_status::text,''),
       COALESCE(to_char(ps.last_avatar_activity AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS'),''),
       COALESCE(ps.previous_server_partition_id::text,'')
FROM dune.player_state ps JOIN dune.accounts a ON a.id = ps.account_id
WHERE a."user"::text = :'fls' LIMIT 1;