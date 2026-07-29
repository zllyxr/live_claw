-- Keep only enabled high-frequency lottery catalog entries.
-- Low-frequency, blockchain/TRX and disabled catalog data must not reappear.

CREATE TEMPORARY TABLE purge_lottery_games (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_lottery_games(id)
SELECT game.id
FROM lottery_games game
JOIN lottery_categories category ON category.id=game.category_id
WHERE game.status<>1
   OR category.status<>1
   OR LOWER(category.category_key) IN ('lf','low','low_frequency','blockchain','tron','trx')
   OR category.name LIKE '%低频%'
   OR category.name LIKE '%波场%'
   OR category.name LIKE '%波厂%'
   OR category.name LIKE '%区块链%'
   OR UPPER(game.game_code) LIKE 'TRX%'
   OR LOWER(game.name) LIKE '%trx%'
   OR game.name LIKE '%波场%'
   OR game.name LIKE '%波厂%';

CREATE TEMPORARY TABLE purge_lottery_plays (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_lottery_plays(id)
SELECT play.id
FROM lottery_plays play
LEFT JOIN purge_lottery_games game ON game.id=play.game_id
WHERE play.status<>1 OR game.id IS NOT NULL;

CREATE TEMPORARY TABLE purge_lottery_issues (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_lottery_issues(id)
SELECT issue.id
FROM lottery_issues issue
JOIN purge_lottery_games game ON game.id=issue.game_id;

CREATE TEMPORARY TABLE purge_lottery_orders (
    id bigint unsigned NOT NULL,
    PRIMARY KEY (id)
) ENGINE=MEMORY;

INSERT INTO purge_lottery_orders(id)
SELECT bet_order.id
FROM lottery_bet_orders bet_order
JOIN purge_lottery_games game ON game.id=bet_order.game_id;

DELETE bet_item
FROM lottery_bet_items bet_item
LEFT JOIN purge_lottery_orders bet_order ON bet_order.id=bet_item.order_id
LEFT JOIN purge_lottery_plays play ON play.id=bet_item.play_id
WHERE bet_order.id IS NOT NULL OR play.id IS NOT NULL;

DELETE bet_order
FROM lottery_bet_orders bet_order
JOIN purge_lottery_orders target ON target.id=bet_order.id;

DELETE settlement
FROM lottery_settlement_runs settlement
JOIN purge_lottery_issues issue ON issue.id=settlement.issue_id;

DELETE audit
FROM lottery_draw_audits audit
JOIN purge_lottery_issues issue ON issue.id=audit.issue_id;

DELETE issue
FROM lottery_issues issue
JOIN purge_lottery_issues target ON target.id=issue.id;

DELETE option_row
FROM lottery_options option_row
LEFT JOIN purge_lottery_plays play ON play.id=option_row.play_id
WHERE option_row.status<>1 OR play.id IS NOT NULL;

DELETE play
FROM lottery_plays play
JOIN purge_lottery_plays target ON target.id=play.id;

DELETE game
FROM lottery_games game
JOIN purge_lottery_games target ON target.id=game.id;

DELETE category
FROM lottery_categories category
LEFT JOIN lottery_games game ON game.category_id=category.id
WHERE category.status<>1
   OR LOWER(category.category_key) IN ('lf','low','low_frequency','blockchain','tron','trx')
   OR category.name LIKE '%低频%'
   OR category.name LIKE '%波场%'
   OR category.name LIKE '%波厂%'
   OR category.name LIKE '%区块链%'
   OR game.id IS NULL;

DELETE mapping
FROM legacy_id_map mapping
LEFT JOIN purge_lottery_games game
  ON mapping.entity_type='lottery_game' AND mapping.legacy_id=CAST(game.id AS CHAR)
LEFT JOIN purge_lottery_plays play
  ON mapping.entity_type='lottery_play' AND mapping.legacy_id=CAST(play.id AS CHAR)
WHERE game.id IS NOT NULL OR play.id IS NOT NULL;

DELETE snapshot
FROM legacy_entity_snapshots snapshot
LEFT JOIN purge_lottery_games game
  ON snapshot.entity_type='lottery_game' AND snapshot.legacy_id=CAST(game.id AS CHAR)
LEFT JOIN purge_lottery_plays play
  ON snapshot.entity_type='lottery_play' AND snapshot.legacy_id=CAST(play.id AS CHAR)
WHERE game.id IS NOT NULL OR play.id IS NOT NULL;

ALTER TABLE lottery_categories
    ADD CONSTRAINT chk_lottery_categories_active CHECK (status=1);

ALTER TABLE lottery_games
    ADD CONSTRAINT chk_lottery_games_active CHECK (status=1);

ALTER TABLE lottery_plays
    ADD CONSTRAINT chk_lottery_plays_active CHECK (status=1);

ALTER TABLE lottery_options
    ADD CONSTRAINT chk_lottery_options_active CHECK (status=1);

DROP TEMPORARY TABLE purge_lottery_orders;
DROP TEMPORARY TABLE purge_lottery_issues;
DROP TEMPORARY TABLE purge_lottery_plays;
DROP TEMPORARY TABLE purge_lottery_games;
