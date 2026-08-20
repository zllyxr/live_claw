ALTER TABLE remote_credential_requests
    CHANGE COLUMN authorization_ciphertext password_ciphertext varbinary(1024) NOT NULL;

ALTER TABLE remote_devices
    CHANGE COLUMN device_code rustdesk_id varchar(80) NOT NULL DEFAULT '';

DELETE FROM schema_migrations WHERE version='0032_remote_control_core.sql';
