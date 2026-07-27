#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DB_CONTAINER="${DB_CONTAINER:-claw-mysql80}"
DB_NAME="${DB_NAME:-claw_live}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-claw_root_pwd}"
MYSQL_CHARSET="${MYSQL_CHARSET:-utf8mb4}"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups}"
TIMESTAMP="$(date +%Y%m%d%H%M%S)"

MYSQL=(docker exec -i "$DB_CONTAINER" mysql --default-character-set="$MYSQL_CHARSET" -u"$DB_USER" -p"$DB_PASSWORD")
MYSQLDUMP=(docker exec "$DB_CONTAINER" mysqldump --default-character-set="$MYSQL_CHARSET" -u"$DB_USER" -p"$DB_PASSWORD" --single-transaction --routines --triggers)

mkdir -p "$BACKUP_DIR"

if docker exec "$DB_CONTAINER" mysql --default-character-set="$MYSQL_CHARSET" -u"$DB_USER" -p"$DB_PASSWORD" -N -e "SHOW DATABASES LIKE '$DB_NAME'" | grep -qx "$DB_NAME"; then
  BACKUP_FILE="$BACKUP_DIR/${DB_NAME}_before_utf8_reimport_${TIMESTAMP}.sql"
  echo "Backing up $DB_NAME to $BACKUP_FILE"
  "${MYSQLDUMP[@]}" "$DB_NAME" > "$BACKUP_FILE"
fi

echo "Recreating database $DB_NAME with $MYSQL_CHARSET"
printf "DROP DATABASE IF EXISTS \`%s\`; CREATE DATABASE \`%s\` DEFAULT CHARACTER SET %s COLLATE utf8mb4_general_ci;\n" "$DB_NAME" "$DB_NAME" "$MYSQL_CHARSET" | "${MYSQL[@]}"

SQL_FILES=(
  "admin/claw_live_20260611_204607.sql"
  "docs/sql/remove_shop_20260606.sql"
  "docs/sql/virtual_live_20260606.sql"
  "docs/sql/lottery_game_20260606.sql"
  "docs/sql/lottery_game_config_20260606.sql"
  "docs/sql/sports_betting_20260610.sql"
  "docs/sql/encoding_repair_20260611.sql"
)

for sql_file in "${SQL_FILES[@]}"; do
  echo "Importing $sql_file"
  "${MYSQL[@]}" "$DB_NAME" < "$sql_file"
done

echo "Verifying charset and key labels"
"${MYSQL[@]}" "$DB_NAME" -e "
SHOW VARIABLES WHERE Variable_name IN ('character_set_server','collation_server','character_set_database','collation_database');
SELECT id, name, HEX(name) AS name_hex FROM cmf_lottery_category WHERE id BETWEEN 1 AND 6 ORDER BY id;
SELECT id, name, remark FROM cmf_admin_menu WHERE app='admin' AND controller='Virtuallive' ORDER BY id;
SELECT id, title, name FROM cmf_auth_rule WHERE LOWER(name) LIKE 'admin/virtuallive/%' ORDER BY id;
"
