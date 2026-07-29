-- Backend v2: administrator sessions and the initial least-privilege RBAC catalog.

CREATE TABLE IF NOT EXISTS admin_sessions (
    id char(26) NOT NULL,
    admin_user_id bigint unsigned NOT NULL,
    token_hash char(64) NOT NULL,
    csrf_hash char(64) NOT NULL,
    ip varchar(45) NOT NULL DEFAULT '',
    user_agent varchar(500) NOT NULL DEFAULT '',
    expires_at datetime(3) NOT NULL,
    last_seen_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_admin_session_token (token_hash),
    KEY idx_admin_sessions_user (admin_user_id, expires_at),
    KEY idx_admin_sessions_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO admin_roles (role_key, name, description, data_scope, status)
VALUES ('super_admin', '超级管理员', '拥有全部后台权限，仅授予平台所有者。', 1, 1)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    description = VALUES(description),
    status = VALUES(status);

INSERT INTO admin_permissions (permission_key, name, module, action, description)
VALUES
    ('dashboard.read', '查看数据统计', 'dashboard', 'read', '查看平台经营和风险统计'),
    ('users.read', '查看用户', 'users', 'read', '查看用户资料、团队和状态'),
    ('users.write', '管理用户', 'users', 'write', '冻结、解冻及调整用户资料'),
    ('wallet.read', '查看资金', 'wallet', 'read', '查看余额、充值、提现和逐局流水'),
    ('wallet.review', '资金审核', 'wallet', 'review', '审核提现与后台调账'),
    ('games.read', '查看游戏', 'games', 'read', '查看游戏、场次、桌位和局记录'),
    ('games.write', '管理游戏', 'games', 'write', '配置游戏、场次倍率和风控'),
    ('live.read', '查看直播', 'live', 'read', '查看抖音直播房间和状态'),
    ('live.write', '管理直播', 'live', 'write', '新增、停用和审核抖音房间'),
    ('app.read', '查看版本', 'app', 'read', '查看原生包和 WGT 发布记录'),
    ('app.write', '管理版本', 'app', 'write', '创建强制更新与静默热更新'),
    ('system.read', '查看系统设置', 'system', 'read', '查看系统设置和审计日志'),
    ('system.write', '管理系统设置', 'system', 'write', '修改系统运行参数'),
    ('rbac.read', '查看权限', 'rbac', 'read', '查看管理员、角色和权限'),
    ('rbac.write', '管理权限', 'rbac', 'write', '管理管理员、角色与授权'),
    ('im.read', '查看 IM', 'im', 'read', '查看单聊、群聊和举报'),
    ('im.moderate', '管理 IM', 'im', 'moderate', '群成员管理、禁言及消息处置'),
    ('lottery.read', '查看彩票', 'lottery', 'read', '查看彩种、期号、订单和开奖'),
    ('lottery.write', '管理彩票', 'lottery', 'write', '配置彩种、玩法、期号与结算')
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    module = VALUES(module),
    action = VALUES(action),
    description = VALUES(description);

INSERT IGNORE INTO admin_role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM admin_roles role
CROSS JOIN admin_permissions permission
WHERE role.role_key = 'super_admin';
