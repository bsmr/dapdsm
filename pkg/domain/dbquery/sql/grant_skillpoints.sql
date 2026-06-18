WITH pid AS (
  SELECT ps.player_controller_id AS player_id
  FROM dune.player_state ps
  JOIN dune.accounts a ON a.id = ps.account_id
  WHERE a."user"::text = :'fls' LIMIT 1
), upd AS (
  UPDATE dune.specialization_tracks t
  SET xp_amount = GREATEST(LEAST(t.xp_amount + :amount::integer, 44182), 0)
  FROM pid
  WHERE t.player_id = pid.player_id AND t.track_type::text = :'track'
  RETURNING t.xp_amount
), ins AS (
  INSERT INTO dune.specialization_tracks (player_id, track_type, xp_amount, level)
  SELECT pid.player_id, :'track'::dune.specializationtracktype,
         GREATEST(LEAST(:amount::integer, 44182), 0), 0::real
  FROM pid
  WHERE NOT EXISTS (SELECT 1 FROM upd)
  RETURNING xp_amount
)
SELECT xp_amount FROM upd UNION ALL SELECT xp_amount FROM ins;