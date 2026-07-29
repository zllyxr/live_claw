-- Backend v2: game catalog, 300-table venue pools, authoritative rounds and settlements.

CREATE TABLE IF NOT EXISTS games (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    game_code varchar(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(100) NOT NULL,
    category varchar(40) NOT NULL,
    cover_asset_id bigint unsigned NOT NULL DEFAULT 0,
    entry_path varchar(255) NOT NULL,
    min_players tinyint unsigned NOT NULL DEFAULT 1,
    max_players tinyint unsigned NOT NULL DEFAULT 1,
    orientation varchar(20) NOT NULL DEFAULT 'auto',
    wallet_enabled tinyint unsigned NOT NULL DEFAULT 1,
    status tinyint unsigned NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0,
    config json NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_games_code (game_code),
    KEY idx_games_status_sort (status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_venues (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    game_id bigint unsigned NOT NULL,
    venue_code varchar(30) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(80) NOT NULL,
    multiplier int unsigned NOT NULL,
    table_count int unsigned NOT NULL DEFAULT 300,
    seats_per_table tinyint unsigned NOT NULL DEFAULT 4,
    min_balance bigint unsigned NOT NULL DEFAULT 0,
    escrow_amount bigint unsigned NOT NULL DEFAULT 0,
    bet_levels json NOT NULL,
    target_rtp_ppm int unsigned NOT NULL DEFAULT 720000,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_game_venue_code (game_id, venue_code),
    KEY idx_game_venues_status (game_id, status, sort_order),
    CHECK (table_count = 300),
    CHECK (seats_per_table = 4),
    CHECK (multiplier IN (1, 5, 10)),
    CHECK (target_rtp_ppm <= 1000000)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_tables (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    game_id bigint unsigned NOT NULL,
    venue_id bigint unsigned NOT NULL,
    table_no smallint unsigned NOT NULL,
    owner_node varchar(100) NOT NULL DEFAULT '',
    lease_until datetime(3) NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    last_active_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_game_table_no (venue_id, table_no),
    KEY idx_game_tables_lease (status, lease_until),
    CHECK (table_no BETWEEN 1 AND 300)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_sessions (
    id char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    game_id bigint unsigned NOT NULL,
    venue_id bigint unsigned NOT NULL,
    table_no smallint unsigned NOT NULL,
    seat_no tinyint unsigned NOT NULL,
    resume_token_hash char(64) NOT NULL,
    escrow_hold_no char(26) NOT NULL DEFAULT '',
    escrow_balance bigint unsigned NOT NULL DEFAULT 0,
    event_seq bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 active, 2 disconnected, 3 settled, 4 expired',
    active_user_key varchar(100) CHARACTER SET ascii COLLATE ascii_bin
        GENERATED ALWAYS AS (
            CASE WHEN status IN (1, 2) THEN CONCAT(user_id, ':', game_id) ELSE NULL END
        ) STORED,
    active_seat_key varchar(100) CHARACTER SET ascii COLLATE ascii_bin
        GENERATED ALWAYS AS (
            CASE WHEN status IN (1, 2) THEN CONCAT(venue_id, ':', table_no, ':', seat_no) ELSE NULL END
        ) STORED,
    connected_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    disconnected_at datetime(3) NULL,
    settled_at datetime(3) NULL,
    expires_at datetime(3) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_game_active_user (active_user_key),
    UNIQUE KEY uk_game_table_seat (active_seat_key),
    KEY idx_game_sessions_table (venue_id, table_no, status),
    KEY idx_game_sessions_expiry (status, expires_at),
    CHECK (table_no BETWEEN 1 AND 300),
    CHECK (seat_no BETWEEN 1 AND 4)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_rounds (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    round_no char(26) NOT NULL,
    game_id bigint unsigned NOT NULL,
    venue_id bigint unsigned NOT NULL,
    table_no smallint unsigned NOT NULL,
    round_seq bigint unsigned NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 waiting, 1 running, 2 settling, 3 settled, 4 cancelled',
    server_seed_hash char(64) NOT NULL DEFAULT '',
    server_seed_ciphertext varbinary(512) NULL,
    started_at datetime(3) NULL,
    ended_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_game_round_no (round_no),
    UNIQUE KEY uk_game_table_round_seq (venue_id, table_no, round_seq),
    KEY idx_game_rounds_status (status, created_at),
    KEY idx_game_rounds_table (venue_id, table_no, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_round_players (
    round_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    seat_no tinyint unsigned NOT NULL,
    hold_no char(26) NOT NULL DEFAULT '',
    buy_in bigint unsigned NOT NULL DEFAULT 0,
    bet_amount bigint unsigned NOT NULL DEFAULT 0,
    payout_amount bigint unsigned NOT NULL DEFAULT 0,
    net_amount bigint NOT NULL DEFAULT 0,
    fee_amount bigint unsigned NOT NULL DEFAULT 0,
    result_code varchar(40) NOT NULL DEFAULT '',
    joined_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    settled_at datetime(3) NULL,
    PRIMARY KEY (round_id, user_id),
    UNIQUE KEY uk_game_round_seat (round_id, seat_no),
    KEY idx_game_round_players_user (user_id, settled_at),
    CHECK (seat_no BETWEEN 1 AND 4)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_events (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    round_id bigint unsigned NULL,
    session_id char(26) NULL,
    game_id bigint unsigned NOT NULL,
    venue_id bigint unsigned NOT NULL,
    table_no smallint unsigned NOT NULL,
    event_seq bigint unsigned NOT NULL,
    event_type varchar(60) NOT NULL,
    user_id bigint unsigned NOT NULL DEFAULT 0,
    amount bigint NOT NULL DEFAULT 0,
    payload json NOT NULL,
    prev_hash char(64) NOT NULL,
    event_hash char(64) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_game_session_event (session_id, event_seq),
    UNIQUE KEY uk_game_round_event (round_id, event_seq),
    UNIQUE KEY uk_game_event_hash (event_hash),
    KEY idx_game_events_round (round_id, event_seq),
    KEY idx_game_events_user (user_id, created_at),
    KEY idx_game_events_table (venue_id, table_no, created_at),
    CHECK ((round_id IS NULL) <> (session_id IS NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_settlements (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    settlement_no char(26) NOT NULL,
    round_id bigint unsigned NULL,
    session_id char(26) NULL,
    game_id bigint unsigned NOT NULL,
    venue_id bigint unsigned NOT NULL,
    table_no smallint unsigned NOT NULL,
    total_bet bigint unsigned NOT NULL DEFAULT 0,
    total_payout bigint unsigned NOT NULL DEFAULT 0,
    platform_fee bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 applied, 2 failed, 3 reversed',
    checksum char(64) NOT NULL,
    applied_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_game_settlement_no (settlement_no),
    UNIQUE KEY uk_game_settlement_round (round_id),
    UNIQUE KEY uk_game_settlement_session (session_id),
    KEY idx_game_settlements_status (status, created_at),
    KEY idx_game_settlements_table (venue_id, table_no, created_at),
    CHECK ((round_id IS NULL) <> (session_id IS NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_settlement_items (
    settlement_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    hold_no char(26) NOT NULL DEFAULT '',
    bet_amount bigint unsigned NOT NULL DEFAULT 0,
    payout_amount bigint unsigned NOT NULL DEFAULT 0,
    fee_amount bigint unsigned NOT NULL DEFAULT 0,
    net_amount bigint NOT NULL DEFAULT 0,
    ledger_entry_no char(26) NULL,
    PRIMARY KEY (settlement_id, user_id),
    UNIQUE KEY uk_game_settlement_ledger (ledger_entry_no),
    KEY idx_game_settlement_items_user (user_id, settlement_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS game_risk_rules (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    game_id bigint unsigned NOT NULL DEFAULT 0,
    venue_id bigint unsigned NOT NULL DEFAULT 0,
    rule_key varchar(80) NOT NULL,
    name varchar(120) NOT NULL,
    config json NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_game_risk_rule (game_id, venue_id, rule_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS fishing_checkpoints (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    session_id char(26) NOT NULL,
    event_seq bigint unsigned NOT NULL,
    escrow_balance bigint unsigned NOT NULL,
    total_cost bigint unsigned NOT NULL,
    total_reward bigint unsigned NOT NULL,
    state_payload json NOT NULL,
    state_hash char(64) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_fishing_checkpoint_seq (session_id, event_seq),
    KEY idx_fishing_checkpoints_session (session_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO games
    (game_code, name, category, entry_path, min_players, max_players, orientation, wallet_enabled, status, sort_order, config)
VALUES
    ('deepsea_hunter', '深海猎手', 'fishing', '/minigame/fish/', 1, 4, 'landscape', 1, 1, 100,
     JSON_OBJECT('match_mode', 'random_table_random_seat', 'authoritative', true))
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    max_players = VALUES(max_players),
    entry_path = VALUES(entry_path),
    config = VALUES(config);

INSERT INTO game_venues
    (game_id, venue_code, name, multiplier, table_count, seats_per_table, min_balance, escrow_amount, bet_levels, target_rtp_ppm, status, sort_order)
SELECT id, 'novice', '新手场', 1, 300, 4, 100, 1000, JSON_ARRAY(1, 2, 5, 10), 720000, 1, 30
FROM games WHERE game_code = 'deepsea_hunter'
ON DUPLICATE KEY UPDATE name = VALUES(name), multiplier = VALUES(multiplier), table_count = 300, seats_per_table = 4;

INSERT INTO game_venues
    (game_id, venue_code, name, multiplier, table_count, seats_per_table, min_balance, escrow_amount, bet_levels, target_rtp_ppm, status, sort_order)
SELECT id, 'expert', '高手场', 5, 300, 4, 500, 5000, JSON_ARRAY(5, 10, 25, 50), 720000, 1, 20
FROM games WHERE game_code = 'deepsea_hunter'
ON DUPLICATE KEY UPDATE name = VALUES(name), multiplier = VALUES(multiplier), table_count = 300, seats_per_table = 4;

INSERT INTO game_venues
    (game_id, venue_code, name, multiplier, table_count, seats_per_table, min_balance, escrow_amount, bet_levels, target_rtp_ppm, status, sort_order)
SELECT id, 'master', '大师场', 10, 300, 4, 1000, 10000, JSON_ARRAY(10, 20, 50, 100), 720000, 1, 10
FROM games WHERE game_code = 'deepsea_hunter'
ON DUPLICATE KEY UPDATE name = VALUES(name), multiplier = VALUES(multiplier), table_count = 300, seats_per_table = 4;
