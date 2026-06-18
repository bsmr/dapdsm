WITH matches AS (
  SELECT DISTINCT
    COALESCE(acct."user"::text,'') AS fls_id,
    COALESCE(ps.character_name,'') AS character_name,
    COALESCE(ps.online_status::text,'') AS online_status,
    COALESCE(to_char(ps.last_avatar_activity AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS'),'') AS last_seen,
    {level_expr} AS player_level,
    a.partition_id
  FROM dune.player_state ps
  LEFT JOIN dune.accounts acct          ON acct.id = ps.account_id
  LEFT JOIN dune.encrypted_accounts enc ON enc.id  = ps.account_id
  LEFT JOIN dune.actors a               ON a.id    = ps.player_pawn_id
  WHERE lower(ps.character_name) LIKE lower(:'q')
     OR lower(convert_from(enc.encrypted_funcom_id,'UTF8')) LIKE lower(:'q')
)
SELECT fls_id, character_name, online_status, last_seen, player_level, partition_id
FROM matches WHERE fls_id <> ''
ORDER BY CASE WHEN lower(online_status)='online' THEN 0 ELSE 1 END, last_seen DESC, character_name ASC
LIMIT :lim;