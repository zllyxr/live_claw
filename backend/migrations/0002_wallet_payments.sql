-- Backend v2: single-writer wallet, payments, withdrawals and reconciliation.

CREATE TABLE IF NOT EXISTS wallet_accounts (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    user_id bigint unsigned NOT NULL,
    currency char(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'COIN',
    available bigint NOT NULL DEFAULT 0,
    frozen bigint NOT NULL DEFAULT 0,
    version bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_wallet_user_currency (user_id, currency),
    KEY idx_wallet_status (status, id),
    CHECK (available >= 0),
    CHECK (frozen >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS wallet_holds (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    hold_no char(26) NOT NULL,
    account_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    business_type varchar(50) NOT NULL,
    business_id varchar(100) NOT NULL,
    amount bigint unsigned NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 active, 1 committed, 2 released, 3 expired',
    expires_at datetime(3) NOT NULL,
    committed_at datetime(3) NULL,
    released_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_wallet_hold_no (hold_no),
    UNIQUE KEY uk_wallet_hold_business (business_type, business_id, user_id),
    KEY idx_wallet_holds_account (account_id, status, created_at),
    KEY idx_wallet_holds_expiry (status, expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS wallet_ledger_entries (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    entry_no char(26) NOT NULL,
    account_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    delta_available bigint NOT NULL DEFAULT 0,
    delta_frozen bigint NOT NULL DEFAULT 0,
    balance_available bigint unsigned NOT NULL,
    balance_frozen bigint unsigned NOT NULL,
    business_type varchar(50) NOT NULL,
    business_id varchar(100) NOT NULL,
    direction tinyint unsigned NOT NULL COMMENT '1 credit, 2 debit, 3 transfer',
    game_code varchar(60) NOT NULL DEFAULT '',
    venue_code varchar(30) NOT NULL DEFAULT '',
    table_no int unsigned NOT NULL DEFAULT 0,
    round_no varchar(80) NOT NULL DEFAULT '',
    description varchar(500) NOT NULL DEFAULT '',
    metadata json NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_wallet_entry_no (entry_no),
    UNIQUE KEY uk_wallet_business_user (business_type, business_id, user_id),
    KEY idx_wallet_ledger_user_time (user_id, created_at, id),
    KEY idx_wallet_ledger_business (business_type, business_id),
    KEY idx_wallet_ledger_round (game_code, round_no, user_id),
    KEY idx_wallet_ledger_created (created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS wallet_adjustments (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    adjustment_no char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    amount bigint NOT NULL,
    reason varchar(500) NOT NULL,
    evidence_asset_id bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 approved, 2 rejected, 3 applied',
    requested_by bigint unsigned NOT NULL,
    reviewed_by bigint unsigned NOT NULL DEFAULT 0,
    reviewed_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_wallet_adjustment_no (adjustment_no),
    KEY idx_wallet_adjustments_status (status, created_at),
    KEY idx_wallet_adjustments_user (user_id, created_at),
    CHECK (amount <> 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS payment_channels (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    channel_key varchar(40) NOT NULL,
    name varchar(100) NOT NULL,
    provider varchar(40) NOT NULL,
    currency char(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    currency_scale tinyint unsigned NOT NULL DEFAULT 2,
    min_amount_minor bigint unsigned NOT NULL DEFAULT 0,
    max_amount_minor bigint unsigned NOT NULL DEFAULT 0,
    config_ciphertext mediumblob NULL,
    key_version int unsigned NOT NULL DEFAULT 1,
    status tinyint unsigned NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_payment_channel_key (channel_key),
    KEY idx_payment_channels_status (status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS recharge_products (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    name varchar(100) NOT NULL,
    fiat_currency char(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    currency_scale tinyint unsigned NOT NULL DEFAULT 2,
    amount_minor bigint unsigned NOT NULL,
    coin_amount bigint unsigned NOT NULL,
    bonus_coin bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_recharge_products_status (status, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS recharge_orders (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    order_no char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    product_id bigint unsigned NOT NULL,
    channel_id bigint unsigned NOT NULL,
    provider_order_no varchar(190) NULL,
    fiat_currency char(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    currency_scale tinyint unsigned NOT NULL,
    amount_minor bigint unsigned NOT NULL,
    coin_amount bigint unsigned NOT NULL,
    bonus_coin bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 created, 1 paying, 2 paid, 3 failed, 4 closed, 5 refunded',
    client_ip varchar(45) NOT NULL DEFAULT '',
    provider_payload json NULL,
    paid_at datetime(3) NULL,
    closed_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_recharge_order_no (order_no),
    UNIQUE KEY uk_recharge_provider_order (channel_id, provider_order_no),
    KEY idx_recharge_user_time (user_id, created_at),
    KEY idx_recharge_status_time (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS withdraw_accounts (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    user_id bigint unsigned NOT NULL,
    account_type varchar(30) NOT NULL,
    account_ciphertext varbinary(1000) NOT NULL,
    account_hash char(64) NOT NULL,
    account_masked varchar(190) NOT NULL,
    holder_name_ciphertext varbinary(512) NULL,
    bank_name varchar(190) NOT NULL DEFAULT '',
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at datetime(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_withdraw_account (user_id, account_type, account_hash),
    KEY idx_withdraw_accounts_user (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS withdraw_orders (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    order_no char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    account_id bigint unsigned NOT NULL,
    hold_no char(26) NOT NULL,
    coin_amount bigint unsigned NOT NULL,
    fee_coin bigint unsigned NOT NULL DEFAULT 0,
    payout_currency char(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    currency_scale tinyint unsigned NOT NULL DEFAULT 2,
    payout_amount_minor bigint unsigned NOT NULL,
    account_snapshot_ciphertext varbinary(1500) NOT NULL,
    account_masked varchar(190) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 approved, 2 paying, 3 paid, 4 rejected, 5 cancelled, 6 failed',
    reject_reason varchar(500) NOT NULL DEFAULT '',
    provider_order_no varchar(190) NOT NULL DEFAULT '',
    requested_ip varchar(45) NOT NULL DEFAULT '',
    reviewed_by bigint unsigned NOT NULL DEFAULT 0,
    reviewed_at datetime(3) NULL,
    paid_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_withdraw_order_no (order_no),
    UNIQUE KEY uk_withdraw_hold_no (hold_no),
    KEY idx_withdraw_user_time (user_id, created_at),
    KEY idx_withdraw_status_time (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS reconciliation_runs (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    run_no char(26) NOT NULL,
    scope varchar(40) NOT NULL,
    date_from date NOT NULL,
    date_to date NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0,
    checked_rows bigint unsigned NOT NULL DEFAULT 0,
    mismatch_rows bigint unsigned NOT NULL DEFAULT 0,
    result json NULL,
    started_at datetime(3) NULL,
    finished_at datetime(3) NULL,
    created_by bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_reconciliation_run_no (run_no),
    KEY idx_reconciliation_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO payment_channels
    (channel_key, name, provider, currency, currency_scale, status, sort_order)
VALUES
    ('ali', '支付宝', 'alipay', 'CNY', 2, 0, 40),
    ('wx', '微信支付', 'wechatpay', 'CNY', 2, 0, 30),
    ('paypal', 'PayPal', 'paypal', 'USD', 2, 0, 20),
    ('usdt', 'USDT.TRC20', 'usdt', 'USDT', 6, 0, 10)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    provider = VALUES(provider),
    currency = VALUES(currency),
    currency_scale = VALUES(currency_scale);
