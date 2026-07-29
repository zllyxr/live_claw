-- Atomic, idempotent live gifts and recoverable IM outbox claims.
-- Every DDL item is guarded independently because MySQL commits ALTER TABLE
-- statements immediately. A retry after a partially applied migration is safe.

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD COLUMN client_request_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER order_no',
        'SELECT 1')
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND column_name = 'client_request_id'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD COLUMN gift_name varchar(100) NOT NULL DEFAULT '''' AFTER gift_id',
        'SELECT 1')
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND column_name = 'gift_name'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD COLUMN gift_icon_url varchar(1000) NOT NULL DEFAULT '''' AFTER gift_name',
        'SELECT 1')
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND column_name = 'gift_icon_url'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD COLUMN credit_ledger_entry_no char(26) NULL AFTER ledger_entry_no',
        'SELECT 1')
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND column_name = 'credit_ledger_entry_no'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD COLUMN im_message_id char(26) NULL AFTER credit_ledger_entry_no',
        'SELECT 1')
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND column_name = 'im_message_id'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD UNIQUE KEY uk_live_gift_client_request (sender_user_id, client_request_id)',
        'SELECT 1')
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND index_name = 'uk_live_gift_client_request'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD UNIQUE KEY uk_live_gift_credit_ledger (credit_ledger_entry_no)',
        'SELECT 1')
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND index_name = 'uk_live_gift_credit_ledger'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE live_gift_orders ADD UNIQUE KEY uk_live_gift_im_message (im_message_id)',
        'SELECT 1')
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'live_gift_orders'
      AND index_name = 'uk_live_gift_im_message'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE outbox_events ADD COLUMN processing_started_at datetime(3) NULL AFTER attempts',
        'SELECT 1')
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'outbox_events'
      AND column_name = 'processing_started_at'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

SET @claw_ddl = (
    SELECT IF(COUNT(*) = 0,
        'ALTER TABLE outbox_events ADD KEY idx_outbox_recover (status, processing_started_at, id)',
        'SELECT 1')
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'outbox_events'
      AND index_name = 'idx_outbox_recover'
);
PREPARE claw_stmt FROM @claw_ddl;
EXECUTE claw_stmt;
DEALLOCATE PREPARE claw_stmt;

-- Preserve existing room-manager grants in the conversation role used by the
-- IM permission check. Offline managers remain inactive until they join.
INSERT INTO im_conversation_members
    (conversation_id,user_id,role,member_status,left_at)
SELECT room.room_no,manager.user_id,60,2,CURRENT_TIMESTAMP(3)
FROM live_room_managers manager
JOIN live_rooms room ON room.id=manager.live_room_id
JOIN users user ON user.id=manager.user_id AND user.status=1
ON DUPLICATE KEY UPDATE
    role=IF(role<100,60,role);
