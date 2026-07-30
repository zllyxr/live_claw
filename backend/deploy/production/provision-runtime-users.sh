#!/bin/sh
set -eu

require_hex_secret() {
  variable_name="$1"
  eval "secret_value=\${$variable_name:-}"
  case "$secret_value" in
    ''|*[!0-9a-fA-F]*)
      echo "$variable_name must be a non-empty hexadecimal secret" >&2
      exit 1
      ;;
  esac
  if [ "${#secret_value}" -lt 32 ]; then
    echo "$variable_name must contain at least 32 hexadecimal characters" >&2
    exit 1
  fi
}

case "${MYSQL_DATABASE:-}" in
  ''|*[!0-9A-Za-z_]*)
    echo "MYSQL_DATABASE must be a simple SQL identifier" >&2
    exit 1
    ;;
esac

require_hex_secret MYSQL_API_PASSWORD
require_hex_secret MYSQL_ADMIN_PASSWORD
require_hex_secret MYSQL_IM_PASSWORD
require_hex_secret MYSQL_GAME_PASSWORD
require_hex_secret MYSQL_SCHEDULER_PASSWORD
require_hex_secret MYSQL_SUPPORT_PASSWORD
require_hex_secret MYSQL_VIRTUAL_PASSWORD

assigned_role_count="$(
  mysql --protocol=TCP -Nse "
    SELECT COUNT(*)
    FROM mysql.role_edges
    WHERE TO_USER IN (
      'claw_api','claw_admin','claw_im',
      'claw_game','claw_scheduler','claw_support','claw_virtual'
    )
  " -h mysql -u root
)"
if [ "$assigned_role_count" != "0" ]; then
  echo "runtime database accounts have unexpected MySQL roles; refusing to continue" >&2
  exit 1
fi

umask 077
sql_file="$(mktemp)"
trap 'rm -f "$sql_file"' EXIT

cat >"$sql_file" <<SQL
CREATE USER IF NOT EXISTS 'claw_api'@'%' IDENTIFIED BY '${MYSQL_API_PASSWORD}';
ALTER USER 'claw_api'@'%' IDENTIFIED BY '${MYSQL_API_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`app_releases\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`daily_tasks\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`game_venues\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`games\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`home_banners\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`live_gifts\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`live_guards\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`lottery_categories\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`lottery_games\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`lottery_issues\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`lottery_options\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`lottery_plays\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`payment_channels\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`recharge_products\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`sports_market_options\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`sports_markets\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`sports_matches\` TO 'claw_api'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`system_settings\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`invite_relations\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`lottery_bet_items\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`lottery_bet_orders\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`media_assets\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`notifications\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`sports_bet_items\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`sports_bet_orders\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`teams\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`wallet_ledger_entries\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`withdraw_orders\` TO 'claw_api'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`douyin_room_profiles\` TO 'claw_api'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`live_rooms\` TO 'claw_api'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`im_moderation_actions\` TO 'claw_api'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`live_moderation_actions\` TO 'claw_api'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`user_reports\` TO 'claw_api'@'%';
GRANT INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`team_members\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`auth_verification_codes\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_conversations\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_conversation_members\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_groups\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_group_applications\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_messages\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`invite_code_aliases\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`live_gift_orders\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`outbox_events\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`payment_callback_block_bindings\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`payment_callback_events\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`recharge_orders\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`social_comments\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`social_posts\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`user_sessions\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`user_task_progress\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`user_verifications\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`users\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`wallet_accounts\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`wallet_holds\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`withdraw_accounts\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, DELETE ON \`${MYSQL_DATABASE}\`.\`live_room_managers\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, DELETE ON \`${MYSQL_DATABASE}\`.\`social_reactions\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, DELETE ON \`${MYSQL_DATABASE}\`.\`user_follows\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`invite_codes\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`live_red_packets\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`live_red_packet_claims\` TO 'claw_api'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`user_blocks\` TO 'claw_api'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`social_post_media\` TO 'claw_api'@'%';

CREATE USER IF NOT EXISTS 'claw_admin'@'%' IDENTIFIED BY '${MYSQL_ADMIN_PASSWORD}';
ALTER USER 'claw_admin'@'%' IDENTIFIED BY '${MYSQL_ADMIN_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'claw_admin'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.* TO 'claw_admin'@'%';

CREATE USER IF NOT EXISTS 'claw_im'@'%' IDENTIFIED BY '${MYSQL_IM_PASSWORD}';
ALTER USER 'claw_im'@'%' IDENTIFIED BY '${MYSQL_IM_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'claw_im'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`users\` TO 'claw_im'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`user_sessions\` TO 'claw_im'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`media_assets\` TO 'claw_im'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_conversations\` TO 'claw_im'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_conversation_members\` TO 'claw_im'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_groups\` TO 'claw_im'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_group_applications\` TO 'claw_im'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`im_messages\` TO 'claw_im'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`im_moderation_actions\` TO 'claw_im'@'%';
GRANT SELECT, INSERT, DELETE ON \`${MYSQL_DATABASE}\`.\`user_blocks\` TO 'claw_im'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`outbox_events\` TO 'claw_im'@'%';

CREATE USER IF NOT EXISTS 'claw_game'@'%' IDENTIFIED BY '${MYSQL_GAME_PASSWORD}';
ALTER USER 'claw_game'@'%' IDENTIFIED BY '${MYSQL_GAME_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'claw_game'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`users\` TO 'claw_game'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`user_sessions\` TO 'claw_game'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`games\` TO 'claw_game'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`game_venues\` TO 'claw_game'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`game_sessions\` TO 'claw_game'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`fishing_checkpoints\` TO 'claw_game'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`wallet_accounts\` TO 'claw_game'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`wallet_holds\` TO 'claw_game'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`wallet_ledger_entries\` TO 'claw_game'@'%';

CREATE USER IF NOT EXISTS 'claw_scheduler'@'%' IDENTIFIED BY '${MYSQL_SCHEDULER_PASSWORD}';
ALTER USER 'claw_scheduler'@'%' IDENTIFIED BY '${MYSQL_SCHEDULER_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'claw_scheduler'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`users\` TO 'claw_scheduler'@'%';
GRANT CREATE TEMPORARY TABLES ON \`${MYSQL_DATABASE}\`.* TO 'claw_scheduler'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`game_settlements\` TO 'claw_scheduler'@'%';
# MySQL locking reads over a joined table require a write-class privilege on
# every locked table even when the statement does not modify that table.
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`lottery_games\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`lottery_issues\` TO 'claw_scheduler'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`lottery_plays\` TO 'claw_scheduler'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`lottery_options\` TO 'claw_scheduler'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`lottery_bet_orders\` TO 'claw_scheduler'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`lottery_bet_items\` TO 'claw_scheduler'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`lottery_draw_audits\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`lottery_settlement_runs\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`sports_matches\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`sports_markets\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`sports_market_options\` TO 'claw_scheduler'@'%';
GRANT SELECT, DELETE ON \`${MYSQL_DATABASE}\`.\`sports_score_events\` TO 'claw_scheduler'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`sports_bet_orders\` TO 'claw_scheduler'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`sports_bet_items\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON \`${MYSQL_DATABASE}\`.\`sports_settlement_runs\` TO 'claw_scheduler'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`sports_sync_logs\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`metric_daily\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`wallet_accounts\` TO 'claw_scheduler'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`wallet_holds\` TO 'claw_scheduler'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`wallet_ledger_entries\` TO 'claw_scheduler'@'%';

CREATE USER IF NOT EXISTS 'claw_support'@'%' IDENTIFIED BY '${MYSQL_SUPPORT_PASSWORD}';
ALTER USER 'claw_support'@'%' IDENTIFIED BY '${MYSQL_SUPPORT_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`users\` TO 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`user_sessions\` TO 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`media_assets\` TO 'claw_support'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`admin_users\` TO 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`admin_roles\` TO 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`admin_permissions\` TO 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`admin_user_roles\` TO 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`admin_role_permissions\` TO 'claw_support'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`admin_sessions\` TO 'claw_support'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`audit_logs\` TO 'claw_support'@'%';
GRANT SELECT, UPDATE ON \`${MYSQL_DATABASE}\`.\`support_agents\` TO 'claw_support'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`support_conversations\` TO 'claw_support'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`support_conversation_reads\` TO 'claw_support'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`support_messages\` TO 'claw_support'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.\`support_quick_replies\` TO 'claw_support'@'%';
GRANT SELECT, INSERT ON \`${MYSQL_DATABASE}\`.\`support_user_notes\` TO 'claw_support'@'%';

CREATE USER IF NOT EXISTS 'claw_virtual'@'%' IDENTIFIED BY '${MYSQL_VIRTUAL_PASSWORD}';
ALTER USER 'claw_virtual'@'%' IDENTIFIED BY '${MYSQL_VIRTUAL_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'claw_virtual'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`users\` TO 'claw_virtual'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`media_assets\` TO 'claw_virtual'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`live_rooms\` TO 'claw_virtual'@'%';
GRANT SELECT, INSERT, UPDATE ON \`${MYSQL_DATABASE}\`.\`douyin_room_profiles\` TO 'claw_virtual'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`wallet_accounts\` TO 'claw_virtual'@'%';
GRANT INSERT ON \`${MYSQL_DATABASE}\`.\`audit_logs\` TO 'claw_virtual'@'%';

SQL

exec mysql --protocol=TCP --default-character-set=utf8mb4 \
  -h mysql -u root "${MYSQL_DATABASE}" <"$sql_file"
