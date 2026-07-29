-- Backend v2: standalone customer-support conversations and permissions.

CREATE TABLE IF NOT EXISTS support_conversations (
    id char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    subject varchar(200) NOT NULL DEFAULT '',
    category varchar(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'general',
    priority tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 normal, 2 high, 3 urgent',
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 waiting, 1 handling, 2 resolved, 3 closed',
    assigned_admin_id bigint unsigned NOT NULL DEFAULT 0,
    active_user_id bigint unsigned
        GENERATED ALWAYS AS (CASE WHEN status IN (0, 1) THEN user_id ELSE NULL END) STORED,
    last_message_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    resolved_at datetime(3) NULL,
    closed_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_support_active_user (active_user_id),
    KEY idx_support_user_time (user_id, created_at),
    KEY idx_support_queue (status, priority, last_message_at),
    KEY idx_support_assignee (assigned_admin_id, status, last_message_at),
    CHECK (priority IN (1, 2, 3)),
    CHECK (status IN (0, 1, 2, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS support_messages (
    id char(26) NOT NULL,
    conversation_id char(26) NOT NULL,
    sender_type tinyint unsigned NOT NULL COMMENT '1 user, 2 support staff, 3 system',
    sender_id bigint unsigned NOT NULL DEFAULT 0,
    client_message_id varchar(100) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    message_type tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 text, 2 image, 3 file',
    text_content varchar(5000) NOT NULL DEFAULT '',
    asset_id bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 normal, 2 withdrawn, 3 deleted',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_support_client_message (sender_type, sender_id, client_message_id),
    KEY idx_support_messages_conversation (conversation_id, created_at, id),
    CHECK (sender_type IN (1, 2, 3)),
    CHECK (message_type IN (1, 2, 3)),
    CHECK (status IN (1, 2, 3))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO admin_permissions (permission_key, name, module, action, description)
VALUES
    ('support.read', '查看客服', 'support', 'read', '查看客服会话、排队状态和历史消息'),
    ('support.write', '处理客服', 'support', 'write', '接单、回复、转派和关闭客服会话')
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    module = VALUES(module),
    action = VALUES(action),
    description = VALUES(description);

INSERT IGNORE INTO admin_role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM admin_roles role
CROSS JOIN admin_permissions permission
WHERE role.role_key = 'super_admin'
  AND permission.permission_key IN ('support.read', 'support.write');
