-- Backend v2: explicit sports and unified betting administration permissions.

INSERT INTO admin_permissions (permission_key, name, module, action, description)
VALUES
    ('sports.read', '查看体育', 'sports', 'read', '查看赛事、盘口、赔率和结算状态'),
    ('sports.write', '管理体育', 'sports', 'write', '新增赛事、维护盘口结果并触发结算'),
    ('bets.read', '查看投注', 'bets', 'read', '统一查看彩票、体育和游戏投注订单')
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
  AND permission.permission_key IN ('sports.read', 'sports.write', 'bets.read');
