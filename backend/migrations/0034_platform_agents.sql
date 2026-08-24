-- Independent platform-agent console, direct permission grants, and team-prefix ownership.

CREATE TABLE IF NOT EXISTS platform_agents (
    admin_user_id bigint unsigned NOT NULL,
    agent_no char(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (admin_user_id),
    UNIQUE KEY uk_platform_agent_no (agent_no),
    KEY idx_platform_agents_status (status, created_at),
    CHECK (status IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS platform_agent_permissions (
    admin_user_id bigint unsigned NOT NULL,
    permission_id bigint unsigned NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (admin_user_id, permission_id),
    KEY idx_platform_agent_permissions_permission (permission_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS platform_agent_teams (
    team_id bigint unsigned NOT NULL,
    admin_user_id bigint unsigned NOT NULL,
    assigned_by bigint unsigned NOT NULL DEFAULT 0,
    assigned_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (team_id),
    KEY idx_platform_agent_teams_agent (admin_user_id, assigned_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO admin_permissions (permission_key, name, module, action, description)
VALUES
    ('agents.read', '查看代理', 'agents', 'read', '查看代理账号、授权和团队前缀归属'),
    ('agents.write', '管理代理', 'agents', 'write', '创建、停用代理及管理授权和团队前缀归属')
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    module = VALUES(module),
    action = VALUES(action),
    description = VALUES(description);

INSERT IGNORE INTO admin_role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM admin_roles role
JOIN admin_permissions permission
  ON permission.permission_key IN ('agents.read', 'agents.write')
WHERE role.role_key = 'super_admin';
