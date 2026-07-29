-- Backend v2: app releases, homepage content, reports and statistics.

CREATE TABLE IF NOT EXISTS app_releases (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    platform varchar(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'android, ios, app',
    release_type varchar(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'native, wgt',
    version_name varchar(40) NOT NULL,
    version_code bigint unsigned NOT NULL,
    min_native_code bigint unsigned NOT NULL DEFAULT 0,
    force_update tinyint unsigned NOT NULL DEFAULT 0,
    silent_update tinyint unsigned NOT NULL DEFAULT 0,
    rollout_percent tinyint unsigned NOT NULL DEFAULT 100,
    package_asset_id bigint unsigned NOT NULL,
    package_size bigint unsigned NOT NULL DEFAULT 0,
    package_sha256 char(64) NOT NULL,
    release_notes varchar(2000) NOT NULL DEFAULT '',
    status tinyint unsigned NOT NULL DEFAULT 0 COMMENT '0 draft, 1 active, 2 paused, 3 retired',
    published_by bigint unsigned NOT NULL DEFAULT 0,
    published_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_app_release_version (platform, release_type, version_code),
    KEY idx_app_releases_check (platform, release_type, status, version_code),
    CHECK (force_update IN (0, 1)),
    CHECK (silent_update IN (0, 1)),
    CHECK (rollout_percent BETWEEN 0 AND 100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE IF NOT EXISTS home_banners (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    title varchar(160) NOT NULL,
    subtitle varchar(300) NOT NULL DEFAULT '',
    image_asset_id bigint unsigned NOT NULL,
    action_type varchar(30) NOT NULL DEFAULT '',
    action_value varchar(500) NOT NULL DEFAULT '',
    audience json NULL,
    starts_at datetime(3) NULL,
    ends_at datetime(3) NULL,
    status tinyint unsigned NOT NULL DEFAULT 1,
    sort_order int NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_home_banners_active (status, starts_at, ends_at, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS content_pages (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    page_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    title varchar(200) NOT NULL,
    content mediumtext NOT NULL,
    content_format varchar(20) NOT NULL DEFAULT 'html',
    status tinyint unsigned NOT NULL DEFAULT 1,
    version bigint unsigned NOT NULL DEFAULT 1,
    updated_by bigint unsigned NOT NULL DEFAULT 0,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_content_page_key (page_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS user_reports (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    reporter_user_id bigint unsigned NOT NULL,
    target_type varchar(30) NOT NULL,
    target_id varchar(100) NOT NULL,
    reason_code varchar(40) NOT NULL,
    description varchar(1000) NOT NULL DEFAULT '',
    evidence json NULL,
    status tinyint unsigned NOT NULL DEFAULT 0,
    handled_by bigint unsigned NOT NULL DEFAULT 0,
    handle_note varchar(1000) NOT NULL DEFAULT '',
    handled_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_user_reports_status (status, created_at),
    KEY idx_user_reports_target (target_type, target_id, created_at),
    KEY idx_user_reports_user (reporter_user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS metric_daily (
    metric_date date NOT NULL,
    metric_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    dimension_key varchar(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
    dimension_value varchar(190) NOT NULL DEFAULT '',
    metric_value bigint NOT NULL DEFAULT 0,
    metadata json NULL,
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (metric_date, metric_key, dimension_key, dimension_value),
    KEY idx_metric_daily_key (metric_key, metric_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS legacy_migration_runs (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    run_no char(26) NOT NULL,
    source_name varchar(80) NOT NULL,
    source_snapshot_at datetime(3) NOT NULL,
    status tinyint unsigned NOT NULL DEFAULT 0,
    phase varchar(40) NOT NULL DEFAULT '',
    migrated_rows bigint unsigned NOT NULL DEFAULT 0,
    skipped_rows bigint unsigned NOT NULL DEFAULT 0,
    mismatch_rows bigint unsigned NOT NULL DEFAULT 0,
    checkpoint json NULL,
    error_message varchar(2000) NOT NULL DEFAULT '',
    started_at datetime(3) NULL,
    finished_at datetime(3) NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_legacy_migration_run (run_no),
    KEY idx_legacy_migration_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS legacy_id_map (
    entity_type varchar(50) NOT NULL,
    legacy_id varchar(100) NOT NULL,
    v2_id varchar(100) NOT NULL,
    migrated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (entity_type, legacy_id),
    UNIQUE KEY uk_legacy_id_v2 (entity_type, v2_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS migration_mismatches (
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    run_id bigint unsigned NOT NULL,
    scope varchar(50) NOT NULL,
    legacy_key varchar(190) NOT NULL,
    expected_value varchar(500) NOT NULL DEFAULT '',
    actual_value varchar(500) NOT NULL DEFAULT '',
    details json NULL,
    resolved_at datetime(3) NULL,
    resolved_by bigint unsigned NOT NULL DEFAULT 0,
    resolution_note varchar(1000) NOT NULL DEFAULT '',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_migration_mismatch_run (run_id, scope, resolved_at),
    KEY idx_migration_mismatch_key (scope, legacy_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
