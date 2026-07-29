-- Backend v2: non-secret operational defaults.
-- These values are deliberately safe for a fresh environment. Payment
-- channels, releases, lottery games and live rooms remain disabled/uncreated
-- until an administrator explicitly configures them.

INSERT INTO system_settings
    (setting_key, setting_value, is_secret, version, updated_by)
VALUES
    ('platform.brand',
     JSON_OBJECT('name', '星域', 'support_url', '', 'maintenance', false),
     0, 1, 0),
    ('security.session',
     JSON_OBJECT('user_ttl_seconds', 2592000, 'admin_ttl_seconds', 28800, 'single_device', false),
     0, 1, 0),
    ('invite.policy',
     JSON_OBJECT(
         'alphabet', '0123456789abcdefghijklmnopqrstuvwxyz',
         'format', 'xxx-xxxx',
         'team_prefix_length', 3,
         'personal_code_length', 4,
         'alias_retention_days', 180
     ),
     0, 1, 0),
    ('wallet.policy',
     JSON_OBJECT(
         'currency', 'COIN',
         'recharge_enabled', false,
         'withdraw_enabled', false,
         'min_withdraw_coin', 100,
         'withdraw_fee_coin', 0
     ),
     0, 1, 0),
    ('game.fishing',
     JSON_OBJECT(
         'allocation', 'random_table_random_seat',
         'tables_per_venue', 300,
         'seats_per_table', 4,
         'venues', JSON_ARRAY(
             JSON_OBJECT('code', 'novice', 'multiplier', 1),
             JSON_OBJECT('code', 'expert', 'multiplier', 5),
             JSON_OBJECT('code', 'master', 'multiplier', 10)
         )
     ),
     0, 1, 0),
    ('live.provider',
     JSON_OBJECT('allowed', JSON_ARRAY('douyin'), 'resolve_cache_seconds', 30),
     0, 1, 0),
    ('im.policy',
     JSON_OBJECT(
         'direct_enabled', true,
         'group_enabled', true,
         'live_group_enabled', true,
         'max_group_members', 5000
     ),
     0, 1, 0),
    ('app.update',
     JSON_OBJECT(
         'force_update_enabled', true,
         'silent_hot_update_enabled', true,
         'rollout_percent', 100
     ),
     0, 1, 0),
    ('lottery.policy',
     JSON_OBJECT('enabled', false, 'manual_draw_requires_audit', true),
     0, 1, 0)
ON DUPLICATE KEY UPDATE setting_key = VALUES(setting_key);
