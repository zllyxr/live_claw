-- Backend v2: isolated customer-support seat console.

SET @support_console_ddl = IF(
    EXISTS(
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA=DATABASE()
          AND TABLE_NAME='admin_sessions'
          AND COLUMN_NAME='portal'
    ),
    'SELECT 1',
    'ALTER TABLE admin_sessions ADD COLUMN portal varchar(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT ''admin'' AFTER admin_user_id'
);
PREPARE support_console_statement FROM @support_console_ddl;
EXECUTE support_console_statement;
DEALLOCATE PREPARE support_console_statement;

SET @support_console_ddl = IF(
    EXISTS(
        SELECT 1 FROM information_schema.STATISTICS
        WHERE TABLE_SCHEMA=DATABASE()
          AND TABLE_NAME='admin_sessions'
          AND INDEX_NAME='idx_admin_sessions_portal'
    ),
    'SELECT 1',
    'ALTER TABLE admin_sessions ADD KEY idx_admin_sessions_portal (portal, expires_at)'
);
PREPARE support_console_statement FROM @support_console_ddl;
EXECUTE support_console_statement;
DEALLOCATE PREPARE support_console_statement;

SET @support_console_ddl = IF(
    EXISTS(
        SELECT 1 FROM information_schema.COLUMNS
        WHERE TABLE_SCHEMA=DATABASE()
          AND TABLE_NAME='support_conversations'
          AND COLUMN_NAME='assigned_at'
    ),
    'SELECT 1',
    'ALTER TABLE support_conversations ADD COLUMN assigned_at datetime(3) NULL AFTER assigned_admin_id'
);
PREPARE support_console_statement FROM @support_console_ddl;
EXECUTE support_console_statement;
DEALLOCATE PREPARE support_console_statement;

CREATE TABLE IF NOT EXISTS support_agents (
    admin_user_id bigint unsigned NOT NULL,
    agent_no char(26) NOT NULL,
    agent_role tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 agent, 2 supervisor',
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 enabled, 0 disabled',
    presence tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 offline, 1 online, 2 away, 3 busy',
    max_active int unsigned NOT NULL DEFAULT 8,
    support_only tinyint unsigned NOT NULL DEFAULT 1,
    last_seen_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (admin_user_id),
    UNIQUE KEY uk_support_agent_no (agent_no),
    KEY idx_support_agents_presence (status, presence, last_seen_at),
    CHECK (agent_role IN (1, 2)),
    CHECK (status IN (0, 1)),
    CHECK (presence IN (0, 1, 2, 3)),
    CHECK (support_only IN (0, 1)),
    CHECK (max_active BETWEEN 1 AND 100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS support_conversation_reads (
    conversation_id char(26) NOT NULL,
    admin_user_id bigint unsigned NOT NULL,
    last_read_message_id char(26) NOT NULL DEFAULT '',
    read_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (conversation_id, admin_user_id),
    KEY idx_support_reads_agent (admin_user_id, read_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS support_user_notes (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    user_id bigint unsigned NOT NULL,
    admin_user_id bigint unsigned NOT NULL,
    content varchar(1000) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_support_notes_user (user_id, created_at),
    KEY idx_support_notes_agent (admin_user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS support_quick_replies (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    title varchar(100) NOT NULL,
    content varchar(1000) NOT NULL,
    category varchar(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'general',
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    created_by bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_support_quick_replies (status, sort_order, id),
    CHECK (status IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO admin_permissions (permission_key, name, module, action, description)
VALUES
    ('support.console', '登录客服座席', 'support', 'console', '登录独立客服座席工作台'),
    ('support.supervise', '客服主管', 'support', 'supervise', '查看全部会话、转接和管理客服座席')
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    module = VALUES(module),
    action = VALUES(action),
    description = VALUES(description);

INSERT INTO admin_roles (role_key, name, description, data_scope, status)
VALUES
    ('support_agent', '客服座席', '仅可登录独立客服座席端并处理本人会话。', 3, 1),
    ('support_supervisor', '客服主管', '可登录独立客服座席端、查看全部会话并执行转接。', 1, 1)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    description = VALUES(description),
    data_scope = VALUES(data_scope),
    status = VALUES(status);

INSERT IGNORE INTO admin_role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM admin_roles role
JOIN admin_permissions permission
  ON permission.permission_key IN ('support.console', 'support.read', 'support.write')
WHERE role.role_key = 'support_agent';

INSERT IGNORE INTO admin_role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM admin_roles role
JOIN admin_permissions permission
  ON permission.permission_key IN ('support.console', 'support.read', 'support.write', 'support.supervise')
WHERE role.role_key = 'support_supervisor';

INSERT INTO support_quick_replies (title, content, category, status, sort_order)
SELECT '欢迎语', '您好，我是本次为您服务的客服，请问有什么可以帮您？', 'general', 1, 100
WHERE NOT EXISTS (SELECT 1 FROM support_quick_replies WHERE title='欢迎语');

INSERT INTO support_quick_replies (title, content, category, status, sort_order)
SELECT '处理中', '您的问题已经收到，我正在为您核实，请稍候。', 'general', 1, 90
WHERE NOT EXISTS (SELECT 1 FROM support_quick_replies WHERE title='处理中');

INSERT INTO support_quick_replies (title, content, category, status, sort_order)
SELECT '结束语', '本次问题已处理完成。如还有其他疑问，欢迎随时联系我们。', 'general', 1, 80
WHERE NOT EXISTS (SELECT 1 FROM support_quick_replies WHERE title='结束语');
