-- Immediate balance adjustments are intentionally stronger than ordinary
-- finance review. Only the protected super-admin role receives this permission
-- automatically; normal reviewers keep the two-person approval workflow.

INSERT INTO admin_permissions (permission_key, name, module, action, description)
VALUES (
    'wallet.adjust',
    '即时调账',
    'wallet',
    'adjust',
    '从用户列表直接充值或扣款；仅限平台所有者并全量记录资金审计'
)
ON DUPLICATE KEY UPDATE
    name = VALUES(name),
    module = VALUES(module),
    action = VALUES(action),
    description = VALUES(description);

INSERT IGNORE INTO admin_role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM admin_roles role
JOIN admin_permissions permission ON permission.permission_key = 'wallet.adjust'
WHERE role.role_key = 'super_admin';
