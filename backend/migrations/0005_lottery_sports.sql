-- Backend v2: lottery and sports remain visible in the current client.

CREATE TABLE IF NOT EXISTS lottery_categories (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    category_key varchar(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(100) NOT NULL,
    icon_asset_id bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_lottery_category_key (category_key),
    KEY idx_lottery_categories_status (status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE IF NOT EXISTS lottery_games (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    category_id bigint unsigned NOT NULL,
    game_code varchar(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(120) NOT NULL,
    icon_asset_id bigint unsigned NOT NULL DEFAULT 0,
    issue_interval_seconds int unsigned NOT NULL,
    sale_close_seconds int unsigned NOT NULL DEFAULT 10,
    min_bet bigint unsigned NOT NULL DEFAULT 1,
    max_bet bigint unsigned NOT NULL DEFAULT 1000000,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    config json NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_lottery_game_code (game_code),
    KEY idx_lottery_games_category (category_id, status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS lottery_plays (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    game_id bigint unsigned NOT NULL,
    play_code varchar(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(120) NOT NULL,
    settlement_rule varchar(80) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    config json NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_lottery_play_code (game_id, play_code),
    KEY idx_lottery_plays_status (game_id, status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS lottery_options (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    play_id bigint unsigned NOT NULL,
    option_code varchar(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(120) NOT NULL,
    odds_scaled bigint unsigned NOT NULL COMMENT 'odds x 1,000,000',
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    config json NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_lottery_option_code (play_id, option_code),
    KEY idx_lottery_options_status (play_id, status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS lottery_issues (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    game_id bigint unsigned NOT NULL,
    issue_no varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    sale_open_at datetime(3) NOT NULL,
    sale_close_at datetime(3) NOT NULL,
    draw_at datetime(3) NOT NULL,
    draw_result json NULL,
    result_source varchar(40) NOT NULL DEFAULT '',
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 open, 2 closed, 3 drawn, 4 settled, 5 cancelled',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_lottery_issue_no (game_id, issue_no),
    KEY idx_lottery_issues_current (game_id, status, sale_close_at),
    KEY idx_lottery_issues_draw (status, draw_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS lottery_bet_orders (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    order_no char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    game_id bigint unsigned NOT NULL,
    issue_id bigint unsigned NOT NULL,
    hold_no char(26) NOT NULL,
    total_bet bigint unsigned NOT NULL,
    total_payout bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 accepted, 1 won, 2 lost, 3 refunded, 4 cancelled',
    client_trace_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    settled_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_lottery_order_no (order_no),
    UNIQUE KEY uk_lottery_client_trace (user_id, client_trace_id),
    UNIQUE KEY uk_lottery_hold_no (hold_no),
    KEY idx_lottery_orders_user (user_id, created_at),
    KEY idx_lottery_orders_issue (issue_id, status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS lottery_bet_items (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    order_id bigint unsigned NOT NULL,
    play_id bigint unsigned NOT NULL,
    option_id bigint unsigned NOT NULL,
    bet_amount bigint unsigned NOT NULL,
    odds_scaled bigint unsigned NOT NULL,
    payout_amount bigint unsigned NOT NULL DEFAULT 0,
    result tinyint unsigned NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_lottery_items_order (order_id),
    KEY idx_lottery_items_option (option_id, order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS lottery_draw_audits (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    issue_id bigint unsigned NOT NULL,
    action varchar(50) NOT NULL,
    source varchar(40) NOT NULL,
    before_result json NULL,
    after_result json NULL,
    payload_hash char(64) NOT NULL DEFAULT '',
    actor_type tinyint unsigned NOT NULL,
    actor_id bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_lottery_draw_audit_issue (issue_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS lottery_settlement_runs (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    run_no char(26) NOT NULL,
    issue_id bigint unsigned NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0,
    order_count bigint unsigned NOT NULL DEFAULT 0,
    total_bet bigint unsigned NOT NULL DEFAULT 0,
    total_payout bigint unsigned NOT NULL DEFAULT 0,
    error_message varchar(1000) NOT NULL DEFAULT '',
    started_at datetime(3) NULL,
    finished_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_lottery_settlement_run (run_no),
    UNIQUE KEY uk_lottery_settlement_issue (issue_id),
    KEY idx_lottery_settlement_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_matches (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    public_match_id char(26) NOT NULL,
    source varchar(40) NOT NULL,
    source_match_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    competition varchar(190) NOT NULL DEFAULT '',
    competition_type varchar(60) NOT NULL DEFAULT '',
    home_name varchar(190) NOT NULL,
    away_name varchar(190) NOT NULL,
    home_logo_url varchar(1000) NOT NULL DEFAULT '',
    away_logo_url varchar(1000) NOT NULL DEFAULT '',
    kickoff_at datetime(3) NOT NULL,
    bet_close_at datetime(3) NOT NULL,
    home_score smallint NOT NULL DEFAULT 0,
    away_score smallint NOT NULL DEFAULT 0,
    match_status varchar(30) NOT NULL DEFAULT 'NS',
    bet_status tinyint unsigned NOT NULL DEFAULT 0,
    settle_status tinyint unsigned NOT NULL DEFAULT 0,
    min_bet bigint unsigned NOT NULL DEFAULT 1,
    max_bet bigint unsigned NOT NULL DEFAULT 500000,
    raw_payload json NULL,
    source_updated_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_sports_public_match (public_match_id),
    UNIQUE KEY uk_sports_source_match (source, source_match_id),
    KEY idx_sports_home (match_status, kickoff_at),
    KEY idx_sports_bettable (bet_status, bet_close_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_markets (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    match_id bigint unsigned NOT NULL,
    market_code varchar(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(120) NOT NULL,
    settlement_rule varchar(80) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    UNIQUE KEY uk_sports_market_code (match_id, market_code),
    KEY idx_sports_markets_status (match_id, status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_market_options (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    market_id bigint unsigned NOT NULL,
    option_code varchar(60) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(120) NOT NULL,
    odds_scaled bigint unsigned NOT NULL COMMENT 'odds x 1,000,000',
    result tinyint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    source_updated_at datetime(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_sports_market_option (market_id, option_code),
    KEY idx_sports_options_status (market_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_bet_orders (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    order_no char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    match_id bigint unsigned NOT NULL,
    hold_no char(26) NOT NULL,
    total_bet bigint unsigned NOT NULL,
    total_payout bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 0,
    client_trace_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    settled_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_sports_order_no (order_no),
    UNIQUE KEY uk_sports_client_trace (user_id, client_trace_id),
    UNIQUE KEY uk_sports_hold_no (hold_no),
    KEY idx_sports_orders_user (user_id, created_at),
    KEY idx_sports_orders_match (match_id, status, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_bet_items (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    order_id bigint unsigned NOT NULL,
    market_id bigint unsigned NOT NULL,
    option_id bigint unsigned NOT NULL,
    bet_amount bigint unsigned NOT NULL,
    odds_scaled bigint unsigned NOT NULL,
    payout_amount bigint unsigned NOT NULL DEFAULT 0,
    result tinyint unsigned NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_sports_items_order (order_id),
    KEY idx_sports_items_option (option_id, order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_settlement_runs (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    run_no char(26) NOT NULL,
    match_id bigint unsigned NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0,
    order_count bigint unsigned NOT NULL DEFAULT 0,
    total_bet bigint unsigned NOT NULL DEFAULT 0,
    total_payout bigint unsigned NOT NULL DEFAULT 0,
    error_message varchar(1000) NOT NULL DEFAULT '',
    started_at datetime(3) NULL,
    finished_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_sports_settlement_run (run_no),
    UNIQUE KEY uk_sports_settlement_match (match_id),
    KEY idx_sports_settlement_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sports_sync_logs (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    sync_type varchar(40) NOT NULL,
    source varchar(40) NOT NULL,
    status tinyint unsigned NOT NULL,
    received_count int unsigned NOT NULL DEFAULT 0,
    changed_count int unsigned NOT NULL DEFAULT 0,
    duration_ms int unsigned NOT NULL DEFAULT 0,
    error_message varchar(1000) NOT NULL DEFAULT '',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_sports_sync_type_time (sync_type, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
