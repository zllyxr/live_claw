-- Serialize publish/resume operations inside each platform and release type so
-- concurrent administrators cannot leave more than one active release.

CREATE TABLE IF NOT EXISTS app_release_lifecycle_locks (
    platform varchar(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    release_type varchar(20) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (platform, release_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
