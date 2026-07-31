-- Manual bank-transfer recharge channel, encrypted receiving accounts, immutable
-- per-order account snapshots, and private payment-proof review.

CREATE TABLE IF NOT EXISTS payment_bank_accounts (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    display_name varchar(100) NOT NULL,
    bank_name varchar(190) NOT NULL,
    branch_name varchar(190) NOT NULL DEFAULT '',
    account_ciphertext mediumblob NOT NULL,
    account_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    account_masked varchar(64) NOT NULL,
    key_version int unsigned NOT NULL DEFAULT 1,
    instructions varchar(500) NOT NULL DEFAULT '',
    status tinyint unsigned NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_payment_bank_account_hash (account_hash),
    KEY idx_payment_bank_accounts_status (status, sort_order, id),
    CHECK (status IN (0,1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS payment_bank_order_details (
    recharge_order_id bigint unsigned NOT NULL,
    bank_account_id bigint unsigned NOT NULL DEFAULT 0,
    account_snapshot_ciphertext mediumblob NULL,
    snapshot_key_version int unsigned NOT NULL DEFAULT 0,
    assigned_by bigint unsigned NOT NULL DEFAULT 0,
    assigned_at datetime(3) NULL,
    close_reason varchar(500) NOT NULL DEFAULT '',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (recharge_order_id),
    KEY idx_bank_order_account (bank_account_id, assigned_at),
    KEY idx_bank_order_assignment (assigned_at, recharge_order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS payment_bank_proofs (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    recharge_order_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    asset_id bigint unsigned NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 approved, 2 rejected',
    review_reason varchar(500) NOT NULL DEFAULT '',
    reviewed_by bigint unsigned NOT NULL DEFAULT 0,
    reviewed_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_bank_proof_order (recharge_order_id),
    KEY idx_bank_proofs_status (status, created_at),
    KEY idx_bank_proofs_user (user_id, created_at),
    CHECK (status IN (0,1,2))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO payment_channels
    (channel_key,name,provider,currency,currency_scale,min_amount_minor,
     max_amount_minor,config_ciphertext,key_version,status,sort_order)
VALUES
    ('bank','银行卡转账','manual_bank','CNY',2,1,0,NULL,1,0,80)
ON DUPLICATE KEY UPDATE channel_key=VALUES(channel_key);
