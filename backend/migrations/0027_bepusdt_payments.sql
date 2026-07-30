-- BEpusdt payment-provider integration, exact callback audit, and client-side
-- idempotency for recharge creation.

-- MySQL DDL auto-commits. Each schema change therefore checks
-- information_schema before executing so a connection loss between an ALTER
-- and the schema_migrations insert can be recovered by replaying this file.
SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'payment_channels'
          AND COLUMN_NAME = 'config_verified_hash'
    ),
    'DO 0',
    'ALTER TABLE `payment_channels` ADD COLUMN `config_verified_hash` char(64) NOT NULL DEFAULT '''' AFTER `key_version`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'payment_channels'
          AND COLUMN_NAME = 'config_verified_at'
    ),
    'DO 0',
    'ALTER TABLE `payment_channels` ADD COLUMN `config_verified_at` datetime(3) NULL AFTER `config_verified_hash`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

-- Switching providers invalidates both the previous ciphertext's meaning and
-- any prior protocol check. The administrator must save the BEpusdt config,
-- pass a signed protocol check, and explicitly enable the channel again.
UPDATE payment_channels
SET min_amount_minor = CASE
        WHEN currency = 'USDT' AND currency_scale = 6 AND min_amount_minor <> 0
            THEN GREATEST(1, CAST(ROUND(min_amount_minor / 10000.0, 0) AS UNSIGNED))
        ELSE min_amount_minor
    END,
    max_amount_minor = CASE
        WHEN currency = 'USDT' AND currency_scale = 6 AND max_amount_minor <> 0
            THEN GREATEST(1, CAST(ROUND(max_amount_minor / 10000.0, 0) AS UNSIGNED))
        ELSE max_amount_minor
    END,
    name = 'USDT.TRC20',
    provider = 'bepusdt',
    currency = 'CNY',
    currency_scale = 2,
    config_verified_hash = '',
    config_verified_at = NULL,
    status = 0
WHERE channel_key = 'usdt'
  AND (
      provider <> 'bepusdt'
      OR currency <> 'CNY'
      OR currency_scale <> 2
  );

-- Older imports sometimes described the selected crypto token as the product
-- currency. For scale=6 rows, dividing minor units by 10^(6-2) preserves the
-- displayed numeric amount at CNY scale=2. Values below half a cent are
-- clamped to one minor unit. All migrated products are disabled for an
-- administrator to review before the BEpusdt channel can be enabled.
UPDATE recharge_products
SET fiat_currency = 'CNY',
    currency_scale = 2,
    amount_minor = GREATEST(
        1,
        CAST(ROUND(amount_minor / 10000.0, 0) AS UNSIGNED)
    ),
    status = 0
WHERE fiat_currency = 'USDT'
  AND currency_scale = 6;

-- A non-standard legacy scale cannot be converted safely without knowing its
-- original business meaning. Keep the raw amount, normalize the schema, and
-- force the product off until it is corrected in the administrator console.
UPDATE recharge_products
SET fiat_currency = 'CNY',
    currency_scale = 2,
    status = 0
WHERE fiat_currency = 'USDT';

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'client_trace_id'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `client_trace_id` varchar(100) NULL AFTER `channel_id`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'provider_config_ciphertext'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `provider_config_ciphertext` mediumblob NULL AFTER `client_trace_id`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'provider_config_key_version'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `provider_config_key_version` int unsigned NOT NULL DEFAULT 0 AFTER `provider_config_ciphertext`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'product_name_snapshot'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `product_name_snapshot` varchar(100) NOT NULL DEFAULT '''' AFTER `provider_config_key_version`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'payment_url'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `payment_url` varchar(1000) NOT NULL DEFAULT '''' AFTER `provider_order_no`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'actual_amount'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `actual_amount` varchar(64) NOT NULL DEFAULT '''' AFTER `payment_url`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'payment_address'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `payment_address` varchar(190) NOT NULL DEFAULT '''' AFTER `actual_amount`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'block_transaction_id'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `block_transaction_id` varchar(190) NOT NULL DEFAULT '''' AFTER `payment_address`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'expires_at'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `expires_at` datetime(3) NULL AFTER `provider_payload`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'callback_count'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `callback_count` int unsigned NOT NULL DEFAULT 0 AFTER `expires_at`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'last_callback_status'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `last_callback_status` tinyint unsigned NOT NULL DEFAULT 0 AFTER `callback_count`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'last_callback_at'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `last_callback_at` datetime(3) NULL AFTER `last_callback_status`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'callback_payload_hash'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `callback_payload_hash` char(64) NOT NULL DEFAULT '''' AFTER `last_callback_at`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND COLUMN_NAME = 'failure_reason'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD COLUMN `failure_reason` varchar(500) NOT NULL DEFAULT '''' AFTER `callback_payload_hash`'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND INDEX_NAME = 'uk_recharge_user_trace'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD UNIQUE KEY `uk_recharge_user_trace` (`user_id`, `client_trace_id`)'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND INDEX_NAME = 'idx_recharge_provider_status'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD KEY `idx_recharge_provider_status` (`channel_id`, `status`, `created_at`)'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

SET @migration_0027_sql = IF(
    EXISTS(
        SELECT 1
        FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA = DATABASE()
          AND TABLE_NAME = 'recharge_orders'
          AND INDEX_NAME = 'idx_recharge_expiry'
    ),
    'DO 0',
    'ALTER TABLE `recharge_orders` ADD KEY `idx_recharge_expiry` (`status`, `expires_at`)'
);
PREPARE migration_0027_stmt FROM @migration_0027_sql;
EXECUTE migration_0027_stmt;
DEALLOCATE PREPARE migration_0027_stmt;

CREATE TABLE IF NOT EXISTS payment_callback_events (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    event_id char(26) NOT NULL,
    channel_id bigint unsigned NOT NULL,
    provider varchar(40) NOT NULL,
    order_no char(26) NOT NULL,
    provider_order_no varchar(190) NOT NULL,
    block_transaction_id varchar(190) NULL,
    provider_status tinyint unsigned NOT NULL,
    payload_hash char(64) NOT NULL,
    signature_valid tinyint unsigned NOT NULL DEFAULT 0,
    process_status tinyint unsigned NOT NULL DEFAULT 0
        COMMENT '0 received, 1 processed, 2 duplicate, 3 rejected',
    error_code varchar(80) NOT NULL DEFAULT '',
    payload json NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    processed_at datetime(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_payment_callback_event_id (event_id),
    UNIQUE KEY uk_payment_callback_payload (channel_id, payload_hash),
    UNIQUE KEY uk_payment_callback_block_status
        (channel_id, block_transaction_id, provider_status),
    KEY idx_payment_callback_order (order_no, created_at),
    KEY idx_payment_callback_status (process_status, created_at),
    CHECK (signature_valid IN (0, 1)),
    CHECK (process_status IN (0, 1, 2, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- This table permanently binds a chain transaction to one local/provider
-- order while the event table above may retain legitimate status progression
-- (for example status 1 then status 2) for the same transaction.
CREATE TABLE IF NOT EXISTS payment_callback_block_bindings (
    channel_id bigint unsigned NOT NULL,
    block_transaction_id varchar(190) NOT NULL,
    order_no char(26) NOT NULL,
    provider_order_no varchar(190) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
        ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (channel_id, block_transaction_id),
    KEY idx_payment_callback_block_order (order_no, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO admin_permissions (permission_key, name, module, action, description)
VALUES
    ('payments.read', '查看支付配置', 'payments', 'read', '查看支付通道、充值档位与回调记录'),
    ('payments.write', '管理支付配置', 'payments', 'write', '配置支付通道密钥、充值档位及通道状态')
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    module = VALUES(module),
    action = VALUES(action),
    description = VALUES(description);

INSERT IGNORE INTO admin_role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM admin_roles role
JOIN admin_permissions permission
  ON permission.permission_key IN ('payments.read', 'payments.write')
WHERE role.role_key = 'super_admin';
