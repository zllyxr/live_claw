-- Destructive rollback for 0031, intended only for a backed-up failed rollout.
-- Stop API/admin first so no process can recreate remote records mid-rollback.

DELETE grant_row FROM admin_role_permissions grant_row
JOIN admin_permissions permission ON permission.id=grant_row.permission_id
WHERE permission.permission_key IN ('remote.read','remote.control','remote.revoke');

DELETE FROM admin_permissions
WHERE permission_key IN ('remote.read','remote.control','remote.revoke');

DROP TABLE IF EXISTS remote_sessions;
DROP TABLE IF EXISTS remote_credential_requests;
DROP TABLE IF EXISTS remote_commands;
DROP TABLE IF EXISTS remote_devices;

DELETE FROM schema_migrations WHERE version='0031_remote_assistance.sql';
