-- Independent credentials created by a platform agent for one owned team.
-- Existing ordinary-user team owner sessions remain valid and compatible.

CREATE TABLE IF NOT EXISTS team_console_accounts (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    team_id bigint unsigned NOT NULL,
    username varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    password_hash varchar(255) NOT NULL,
    display_name varchar(100) NOT NULL DEFAULT '',
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 enabled, 0 disabled',
    created_by bigint unsigned NOT NULL DEFAULT 0 COMMENT 'platform agent admin_user_id',
    last_login_at datetime(3) NULL,
    last_login_ip varchar(45) NOT NULL DEFAULT '',
    password_changed_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_team_console_accounts_team (team_id),
    UNIQUE KEY uk_team_console_accounts_username (username),
    KEY idx_team_console_accounts_status (status, updated_at),
    CHECK (status IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE team_console_sessions
    MODIFY COLUMN user_id bigint unsigned NULL,
    ADD COLUMN account_id bigint unsigned NULL AFTER user_id,
    ADD KEY idx_team_console_sessions_account (account_id, expires_at),
    ADD CONSTRAINT chk_team_console_session_principal
        CHECK ((user_id IS NOT NULL AND account_id IS NULL)
            OR (user_id IS NULL AND account_id IS NOT NULL));

ALTER TABLE audit_logs
    MODIFY COLUMN actor_type tinyint unsigned NOT NULL
        COMMENT '1 admin, 2 user, 3 system, 4 team account';
