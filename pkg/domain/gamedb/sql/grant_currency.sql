SELECT dune.adjust_player_virtual_currency_balance(
  (SELECT ps.player_controller_id FROM dune.player_state ps
     JOIN dune.accounts a ON a.id = ps.account_id
     WHERE a."user"::text = :'fls' LIMIT 1),
  :currency::smallint, :delta::bigint);