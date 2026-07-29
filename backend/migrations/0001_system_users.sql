-- Backend v2: system, administrators, users, teams and visible social features.
-- Target database: claw_v2 (selected by the migration runner).

CREATE TABLE IF NOT EXISTS schema_migrations (
    version varchar(100) NOT NULL,
    checksum char(64) NOT NULL,
    applied_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS system_settings (
    setting_key varchar(120) NOT NULL,
    setting_value json NOT NULL,
    is_secret tinyint unsigned NOT NULL DEFAULT 0,
    version bigint unsigned NOT NULL DEFAULT 1,
    updated_by bigint unsigned NOT NULL DEFAULT 0,
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (setting_key),
    CHECK (is_secret IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS audit_logs (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    request_id char(26) NOT NULL,
    actor_type tinyint unsigned NOT NULL COMMENT '1 admin, 2 user, 3 system',
    actor_id bigint unsigned NOT NULL DEFAULT 0,
    action varchar(120) NOT NULL,
    resource_type varchar(80) NOT NULL DEFAULT '',
    resource_id varchar(100) NOT NULL DEFAULT '',
    before_data json NULL,
    after_data json NULL,
    ip varchar(45) NOT NULL DEFAULT '',
    user_agent varchar(500) NOT NULL DEFAULT '',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_audit_actor_time (actor_type, actor_id, created_at),
    KEY idx_audit_resource (resource_type, resource_id, created_at),
    KEY idx_audit_request (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS outbox_events (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    event_id char(26) NOT NULL,
    aggregate_type varchar(80) NOT NULL,
    aggregate_id varchar(100) NOT NULL,
    event_type varchar(120) NOT NULL,
    payload json NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 processing, 2 done, 3 dead',
    attempts int unsigned NOT NULL DEFAULT 0,
    available_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    processed_at datetime(3) NULL,
    last_error varchar(1000) NOT NULL DEFAULT '',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_outbox_event (event_id),
    KEY idx_outbox_pick (status, available_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS admin_users (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    username varchar(80) NOT NULL,
    password_hash varchar(255) NOT NULL,
    display_name varchar(100) NOT NULL DEFAULT '',
    email varchar(190) NULL,
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 enabled, 0 disabled',
    totp_secret_ciphertext varbinary(512) NULL,
    last_login_at datetime(3) NULL,
    last_login_ip varchar(45) NOT NULL DEFAULT '',
    password_changed_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_admin_username (username),
    UNIQUE KEY uk_admin_email (email),
    CHECK (status IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS admin_roles (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    role_key varchar(80) NOT NULL,
    name varchar(100) NOT NULL,
    description varchar(500) NOT NULL DEFAULT '',
    data_scope tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 all, 2 team, 3 self',
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_admin_role_key (role_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS admin_permissions (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    permission_key varchar(120) NOT NULL,
    name varchar(120) NOT NULL,
    module varchar(60) NOT NULL,
    action varchar(60) NOT NULL,
    description varchar(500) NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    UNIQUE KEY uk_admin_permission_key (permission_key),
    KEY idx_admin_permission_module (module, action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS admin_user_roles (
    admin_user_id bigint unsigned NOT NULL,
    role_id bigint unsigned NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (admin_user_id, role_id),
    KEY idx_admin_user_roles_role (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS admin_role_permissions (
    role_id bigint unsigned NOT NULL,
    permission_id bigint unsigned NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (role_id, permission_id),
    KEY idx_admin_role_permissions_permission (permission_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS users (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    username varchar(120) NOT NULL,
    country_code varchar(8) NOT NULL DEFAULT '86',
    mobile varchar(32) NULL,
    email varchar(190) NULL,
    password_hash varchar(255) NOT NULL,
    password_algo varchar(30) NOT NULL DEFAULT 'argon2id',
    nickname varchar(100) NOT NULL DEFAULT '',
    avatar_asset_id bigint unsigned NOT NULL DEFAULT 0,
    background_asset_id bigint unsigned NOT NULL DEFAULT 0,
    gender tinyint unsigned NOT NULL DEFAULT 0,
    birthday date NULL,
    signature varchar(500) NOT NULL DEFAULT '',
    team_id bigint unsigned NOT NULL DEFAULT 0,
    tier_id bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 enabled, 2 frozen, 3 closed',
    is_virtual tinyint unsigned NOT NULL DEFAULT 0,
    registered_ip varchar(45) NOT NULL DEFAULT '',
    last_login_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    closed_at datetime(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_login (country_code, username),
    UNIQUE KEY uk_users_mobile (country_code, mobile),
    UNIQUE KEY uk_users_email (email),
    KEY idx_users_team (team_id, id),
    KEY idx_users_status_created (status, created_at),
    CHECK (status IN (1, 2, 3)),
    CHECK (is_virtual IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_sessions (
    id char(26) NOT NULL,
    user_id bigint unsigned NOT NULL,
    token_hash char(64) NOT NULL,
    device_id varchar(190) NOT NULL DEFAULT '',
    platform varchar(20) NOT NULL DEFAULT '',
    ip varchar(45) NOT NULL DEFAULT '',
    user_agent varchar(500) NOT NULL DEFAULT '',
    expires_at datetime(3) NOT NULL,
    last_seen_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_session_token (token_hash),
    KEY idx_user_sessions_user (user_id, expires_at),
    KEY idx_user_sessions_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_devices (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    user_id bigint unsigned NOT NULL,
    device_id varchar(190) NOT NULL,
    platform varchar(20) NOT NULL,
    push_token varchar(500) NOT NULL DEFAULT '',
    app_version_code bigint unsigned NOT NULL DEFAULT 0,
    last_ip varchar(45) NOT NULL DEFAULT '',
    last_seen_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_device (user_id, device_id),
    KEY idx_user_devices_device (device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_verifications (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    user_id bigint unsigned NOT NULL,
    real_name_ciphertext varbinary(512) NOT NULL,
    document_no_ciphertext varbinary(512) NOT NULL,
    document_hash char(64) NOT NULL,
    front_asset_id bigint unsigned NOT NULL DEFAULT 0,
    back_asset_id bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 pending, 1 approved, 2 rejected',
    reject_reason varchar(500) NOT NULL DEFAULT '',
    reviewed_by bigint unsigned NOT NULL DEFAULT 0,
    reviewed_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_verification_user (user_id),
    UNIQUE KEY uk_user_verification_document (document_hash),
    KEY idx_user_verification_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_tiers (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    tier_key varchar(60) NOT NULL,
    name varchar(100) NOT NULL,
    level int unsigned NOT NULL DEFAULT 0,
    rules json NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_tier_key (tier_key),
    UNIQUE KEY uk_user_tier_level (level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS teams (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    code char(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    name varchar(100) NOT NULL,
    owner_user_id bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_by bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_teams_code (code),
    CHECK (code REGEXP '^[0-9a-z]{3}$'),
    CHECK (status IN (0, 1))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS team_members (
    user_id bigint unsigned NOT NULL,
    team_id bigint unsigned NOT NULL,
    inviter_user_id bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    joined_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    left_at datetime(3) NULL,
    PRIMARY KEY (user_id),
    KEY idx_team_members_team (team_id, joined_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS invite_codes (
    user_id bigint unsigned NOT NULL,
    team_id bigint unsigned NOT NULL,
    team_code char(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    personal_code char(4) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    full_code char(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id),
    UNIQUE KEY uk_invite_full_code (full_code),
    UNIQUE KEY uk_invite_team_personal (team_id, personal_code),
    CHECK (team_code REGEXP '^[0-9a-z]{3}$'),
    CHECK (personal_code REGEXP '^[0-9a-z]{4}$'),
    CHECK (full_code REGEXP '^[0-9a-z]{3}-[0-9a-z]{4}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS invite_code_aliases (
    alias_code varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    user_id bigint unsigned NOT NULL,
    expires_at datetime(3) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (alias_code),
    KEY idx_invite_alias_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS invite_relations (
    invitee_user_id bigint unsigned NOT NULL,
    inviter_user_id bigint unsigned NOT NULL,
    team_id bigint unsigned NOT NULL,
    invite_code char(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    source varchar(30) NOT NULL,
    confidence tinyint unsigned NOT NULL DEFAULT 100,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (invitee_user_id),
    KEY idx_invite_relations_inviter (inviter_user_id, created_at),
    KEY idx_invite_relations_team (team_id, created_at),
    CHECK (invitee_user_id <> inviter_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS invite_clicks (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    click_id char(26) NOT NULL,
    invite_code varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    inviter_user_id bigint unsigned NOT NULL DEFAULT 0,
    matched_user_id bigint unsigned NOT NULL DEFAULT 0,
    platform varchar(20) NOT NULL DEFAULT '',
    device_fingerprint_hash char(64) NOT NULL DEFAULT '',
    ip_hash char(64) NOT NULL DEFAULT '',
    user_agent_hash char(64) NOT NULL DEFAULT '',
    match_method varchar(30) NOT NULL DEFAULT '',
    confidence tinyint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 0,
    expires_at datetime(3) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_invite_click_id (click_id),
    KEY idx_invite_click_code (invite_code, created_at),
    KEY idx_invite_click_fingerprint (device_fingerprint_hash, created_at),
    KEY idx_invite_click_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_follows (
    user_id bigint unsigned NOT NULL,
    target_user_id bigint unsigned NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, target_user_id),
    KEY idx_user_follows_target (target_user_id, created_at),
    CHECK (user_id <> target_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_blocks (
    user_id bigint unsigned NOT NULL,
    target_user_id bigint unsigned NOT NULL,
    reason varchar(255) NOT NULL DEFAULT '',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, target_user_id),
    KEY idx_user_blocks_target (target_user_id),
    CHECK (user_id <> target_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS media_assets (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    owner_user_id bigint unsigned NOT NULL DEFAULT 0,
    bucket varchar(80) NOT NULL,
    object_key varchar(500) NOT NULL,
    media_type varchar(30) NOT NULL,
    mime_type varchar(120) NOT NULL,
    size_bytes bigint unsigned NOT NULL DEFAULT 0,
    width int unsigned NOT NULL DEFAULT 0,
    height int unsigned NOT NULL DEFAULT 0,
    duration_ms bigint unsigned NOT NULL DEFAULT 0,
    sha256 char(64) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    deleted_at datetime(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_media_object (bucket, object_key),
    KEY idx_media_owner (owner_user_id, created_at),
    KEY idx_media_sha256 (sha256)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS social_posts (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    user_id bigint unsigned NOT NULL,
    post_type tinyint unsigned NOT NULL COMMENT '0 text, 1 image, 2 video, 3 voice',
    content varchar(5000) NOT NULL DEFAULT '',
    visibility tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 public, 2 followers, 3 private',
    status tinyint unsigned NOT NULL DEFAULT 1 COMMENT '1 published, 2 hidden, 3 deleted',
    like_count int unsigned NOT NULL DEFAULT 0,
    comment_count int unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at datetime(3) NULL,
    PRIMARY KEY (id),
    KEY idx_social_posts_feed (status, visibility, created_at),
    KEY idx_social_posts_user (user_id, status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS social_post_media (
    post_id bigint unsigned NOT NULL,
    asset_id bigint unsigned NOT NULL,
    sort_order smallint unsigned NOT NULL DEFAULT 0,
    PRIMARY KEY (post_id, asset_id),
    KEY idx_social_post_media_sort (post_id, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS social_comments (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    post_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    parent_comment_id bigint unsigned NOT NULL DEFAULT 0,
    reply_to_user_id bigint unsigned NOT NULL DEFAULT 0,
    content varchar(2000) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    like_count int unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    deleted_at datetime(3) NULL,
    PRIMARY KEY (id),
    KEY idx_social_comments_post (post_id, status, created_at),
    KEY idx_social_comments_parent (parent_comment_id, created_at),
    KEY idx_social_comments_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS social_reactions (
    target_type tinyint unsigned NOT NULL COMMENT '1 post, 2 comment',
    target_id bigint unsigned NOT NULL,
    user_id bigint unsigned NOT NULL,
    reaction tinyint unsigned NOT NULL DEFAULT 1,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (target_type, target_id, user_id),
    KEY idx_social_reactions_user (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS notifications (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    user_id bigint unsigned NOT NULL,
    notification_type varchar(40) NOT NULL,
    actor_user_id bigint unsigned NOT NULL DEFAULT 0,
    title varchar(200) NOT NULL DEFAULT '',
    content varchar(1000) NOT NULL DEFAULT '',
    payload json NULL,
    read_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_notifications_user (user_id, read_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS daily_tasks (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    task_key varchar(80) NOT NULL,
    name varchar(120) NOT NULL,
    description varchar(500) NOT NULL DEFAULT '',
    target_count int unsigned NOT NULL DEFAULT 1,
    reward_coin bigint unsigned NOT NULL DEFAULT 0,
    status tinyint unsigned NOT NULL DEFAULT 1,
    rules json NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_daily_task_key (task_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_task_progress (
    user_id bigint unsigned NOT NULL,
    task_id bigint unsigned NOT NULL,
    task_date date NOT NULL,
    progress int unsigned NOT NULL DEFAULT 0,
    completed_at datetime(3) NULL,
    claimed_at datetime(3) NULL,
    PRIMARY KEY (user_id, task_id, task_date),
    KEY idx_user_task_date (task_date, task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO teams (code, name, status, created_by)
VALUES ('sys', '系统默认团队', 1, 0)
ON DUPLICATE KEY UPDATE name = VALUES(name), status = VALUES(status);
