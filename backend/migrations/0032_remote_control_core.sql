-- Rename legacy vendor-specific fields after moving to the in-house control core.

ALTER TABLE remote_devices
    CHANGE COLUMN rustdesk_id device_code varchar(80) NOT NULL DEFAULT '';

ALTER TABLE remote_credential_requests
    CHANGE COLUMN password_ciphertext authorization_ciphertext varbinary(1024) NOT NULL;

UPDATE admin_permissions
SET name='发起远程控制', description='复核管理员身份并签发短时远程控制授权'
WHERE permission_key='remote.control';
