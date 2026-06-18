BEGIN;
INSERT INTO dune.encrypted_accounts (id, "user", encrypted_funcom_id, takeoverable, platform_id, platform_name)
VALUES (:acct, :'hex', dune.encrypt_user_data(:'funcom'), false, 'dunemgr', 'Dunemgr')
ON CONFLICT DO NOTHING;
INSERT INTO dune.actors (id, class, map, partition_id, dimension_index, gas_attributes, properties, owner_account_id, serial)
VALUES (:ctrl, :'cclass', :'map', :part, :dim, '{}'::jsonb, '{}'::jsonb, :acct, 1) ON CONFLICT DO NOTHING;
INSERT INTO dune.actors (id, class, map, partition_id, dimension_index, gas_attributes, properties, owner_account_id, serial)
VALUES (:state, :'sclass', :'map', :part, :dim, '{}'::jsonb, '{}'::jsonb, :acct, 1) ON CONFLICT DO NOTHING;
INSERT INTO dune.actors (id, class, map, partition_id, dimension_index, gas_attributes, properties, owner_account_id, serial)
VALUES (:pawn, :'pclass', :'map', :part, :dim, '{}'::jsonb, '{}'::jsonb, :acct, 1) ON CONFLICT DO NOTHING;
INSERT INTO dune.encrypted_player_state
  (account_id, encrypted_character_name, life_state, online_status, is_coriolis_processed,
   server_id, player_controller_id, player_pawn_id, player_state_id, last_login_time)
VALUES (:acct, dune.encrypt_user_data(:'name'), :'life'::dune.playerlifestate, :'online'::dune.playerconnectionstatus, false,
   (SELECT server_id FROM dune.encrypted_player_state WHERE server_id IS NOT NULL LIMIT 1),
   :ctrl, :pawn, :state, now())
ON CONFLICT DO NOTHING;
COMMIT;