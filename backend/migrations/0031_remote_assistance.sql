-- User-consented RustDesk Android host enrollment, command delivery and audit state.

CREATE TABLE IF NOT EXISTS remote_devices (
    id char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    install_id varchar(190) NOT NULL,
    device_token_hash char(64) NULL,
    device_name varchar(120) NOT NULL DEFAULT '',
    manufacturer varchar(80) NOT NULL DEFAULT '',
    model varchar(120) NOT NULL DEFAULT '',
    android_version varchar(32) NOT NULL DEFAULT '',
    android_sdk smallint unsigned NOT NULL DEFAULT 0,
    app_version varchar(40) NOT NULL DEFAULT '',
    app_native_code int unsigned NOT NULL DEFAULT 0,
    plugin_version varchar(40) NOT NULL DEFAULT '',
    rustdesk_id varchar(80) NOT NULL DEFAULT '',
    service_status varchar(32) NOT NULL DEFAULT 'stopped',
    permission_status json NULL,
    capabilities json NULL,
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 active, 2 revoking, 3 revoked',
    last_seen_at datetime(3) NULL,
    revoked_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_remote_device_install (install_id),
    UNIQUE KEY uk_remote_device_token (device_token_hash),
    KEY idx_remote_device_user (user_id,status),
    KEY idx_remote_device_online (status,last_seen_at),
    CONSTRAINT fk_remote_device_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS remote_commands (
    id char(26) NOT NULL,
    remote_device_id char(26) NOT NULL,
    command_type varchar(40) NOT NULL,
    payload_ciphertext varbinary(8192) NULL,
    status varchar(20) NOT NULL DEFAULT 'pending',
    expires_at datetime(3) NOT NULL,
    delivered_at datetime(3) NULL,
    acknowledged_at datetime(3) NULL,
    result_data json NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_remote_command_poll (remote_device_id,status,expires_at,created_at),
    CONSTRAINT fk_remote_command_device FOREIGN KEY (remote_device_id) REFERENCES remote_devices(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS remote_credential_requests (
    id char(26) NOT NULL,
    remote_device_id char(26) NOT NULL,
    requested_by bigint unsigned NOT NULL,
    command_id char(26) NOT NULL,
    password_ciphertext varbinary(1024) NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'pending',
    expires_at datetime(3) NOT NULL,
    ready_at datetime(3) NULL,
    revealed_at datetime(3) NULL,
    consumed_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_remote_credential_command (command_id),
    KEY idx_remote_credential_device (remote_device_id,status,expires_at),
    CONSTRAINT fk_remote_credential_device FOREIGN KEY (remote_device_id) REFERENCES remote_devices(id),
    CONSTRAINT fk_remote_credential_admin FOREIGN KEY (requested_by) REFERENCES admin_users(id),
    CONSTRAINT fk_remote_credential_command FOREIGN KEY (command_id) REFERENCES remote_commands(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS remote_sessions (
    id char(26) NOT NULL,
    remote_device_id char(26) NOT NULL,
    credential_request_id char(26) NULL,
    event_type varchar(40) NOT NULL,
    session_ref varchar(100) NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'reported',
    metadata json NULL,
    occurred_at datetime(3) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_remote_session_device (remote_device_id,occurred_at),
    CONSTRAINT fk_remote_session_device FOREIGN KEY (remote_device_id) REFERENCES remote_devices(id),
    CONSTRAINT fk_remote_session_credential FOREIGN KEY (credential_request_id) REFERENCES remote_credential_requests(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO admin_permissions (permission_key,name,module,action,description)
VALUES
    ('remote.read','查看远程设备','remote','read','查看用户主动开启的远程协助设备与状态'),
    ('remote.control','发起远程协助','remote','control','签发一次性远程协助临时凭据'),
    ('remote.revoke','停用远程协助','remote','revoke','撤销设备凭据并停止远程协助')
ON DUPLICATE KEY UPDATE
    name=VALUES(name),module=VALUES(module),action=VALUES(action),description=VALUES(description);

INSERT IGNORE INTO admin_role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM admin_roles role
CROSS JOIN admin_permissions permission
WHERE role.role_key='super_admin' AND permission.permission_key LIKE 'remote.%';
