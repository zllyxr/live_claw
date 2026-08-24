-- Independent team-owner console sessions. Team ownership itself remains on teams.owner_user_id.

CREATE TABLE IF NOT EXISTS team_console_sessions (
    id char(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id bigint unsigned NOT NULL,
    token_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    csrf_hash char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    ip varchar(45) NOT NULL DEFAULT '',
    user_agent varchar(500) NOT NULL DEFAULT '',
    expires_at datetime(3) NOT NULL,
    last_seen_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_team_console_session_token (token_hash),
    KEY idx_team_console_sessions_user (user_id, expires_at),
    KEY idx_team_console_sessions_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
